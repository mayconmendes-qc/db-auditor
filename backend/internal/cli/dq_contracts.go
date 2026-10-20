package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/osmendes/db-auditor/internal/config"
	"github.com/osmendes/db-auditor/internal/dqcontract"
)

func runDQContracts(args []string) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprintln(os.Stderr, `Uso: auditor dq-contracts run --environment <id> (--contract <id> | --file c.yaml | --dir contracts/) [--fail-on high] [--format md|json]

Autentica com AUDITOR_ENV_TOKEN / AUDITOR_ENV_TOKEN_FILE (escopo quality-contract:run).
A CLI não se conecta ao banco alvo: pede a execução à API.`)
		return ExitOK
	}
	if args[0] != "run" {
		fmt.Fprintf(os.Stderr, "subcomando desconhecido %q (use run)\n", args[0])
		return ExitUsage
	}
	fs := flag.NewFlagSet("dq-contracts run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	envID := fs.String("environment", "", "UUID do ambiente")
	contractID := fs.String("contract", "", "UUID do contrato aprovado")
	file := fs.String("file", "", "YAML ad hoc (execução cli; não aprova)")
	dir := fs.String("dir", "", "diretório de YAMLs ad hoc")
	failOn := fs.String("fail-on", "high", "severidade mínima para código 1")
	format := fs.String("format", "md", "md ou json")
	apiURL := fs.String("api-url", "", "URL base da API (ou AUDITOR_API_URL)")
	if err := fs.Parse(args[1:]); err != nil {
		return ExitUsage
	}
	if err := RefuseSecretFlags(args); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return ExitUsage
	}
	if strings.TrimSpace(*envID) == "" {
		fmt.Fprintln(os.Stderr, "informe --environment <uuid>")
		return ExitUsage
	}
	sources := 0
	if *contractID != "" {
		sources++
	}
	if *file != "" {
		sources++
	}
	if *dir != "" {
		sources++
	}
	if sources != 1 {
		fmt.Fprintln(os.Stderr, "informe exatamente um de --contract, --file ou --dir")
		return ExitUsage
	}
	fmtName, err := FormatFlag(*format)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return ExitUsage
	}
	token, err := ReadSecret("AUDITOR_ENV_TOKEN", "AUDITOR_ENV_TOKEN_FILE")
	if err != nil || strings.TrimSpace(token) == "" {
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
		} else {
			fmt.Fprintln(os.Stderr, "informe AUDITOR_ENV_TOKEN ou AUDITOR_ENV_TOKEN_FILE")
		}
		return ExitUsage
	}
	base := strings.TrimSpace(*apiURL)
	if base == "" {
		base = strings.TrimSpace(os.Getenv("AUDITOR_API_URL"))
	}
	client := NewAPIClient(base, token, 120*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	var reports []dqcontract.RunReport
	gateCode := ExitOK

	if *contractID != "" {
		rep, code := dqRunRemote(ctx, client, *envID, *contractID, nil, *failOn)
		if code == ExitUsage || code == ExitSecurityRefused {
			return code
		}
		gateCode = dqMergeGate(gateCode, code)
		reports = append(reports, rep)
	} else {
		files, err := dqCollectYAMLFiles(*file, *dir)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return ExitUsage
		}
		for _, path := range files {
			raw, err := os.ReadFile(path)
			if err != nil {
				fmt.Fprintln(os.Stderr, config.SanitizeError(err))
				return ExitUsage
			}
			if _, _, err := dqcontract.ParseYAML(raw, dqcontract.ProcessMaxSample()); err != nil {
				fmt.Fprintf(os.Stderr, "%s: %s\n", path, err.Error())
				return ExitUsage
			}
			rep, code := dqRunRemote(ctx, client, *envID, "", raw, *failOn)
			if code == ExitUsage || code == ExitSecurityRefused {
				return code
			}
			gateCode = dqMergeGate(gateCode, code)
			reports = append(reports, rep)
		}
	}

	if fmtName == "json" {
		if len(reports) == 1 {
			if err := WriteJSON(os.Stdout, reports[0]); err != nil {
				fmt.Fprintln(os.Stderr, err.Error())
				return ExitUsage
			}
		} else if err := WriteJSON(os.Stdout, reports); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return ExitUsage
		}
	} else {
		var md strings.Builder
		for i, rep := range reports {
			if i > 0 {
				md.WriteString("\n---\n\n")
			}
			md.WriteString(dqcontract.RenderMarkdown(rep))
		}
		if err := WriteMarkdown(os.Stdout, md.String()); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return ExitUsage
		}
	}
	return gateCode
}

func dqMergeGate(cur, next int) int {
	if next == ExitBlocked {
		return ExitBlocked
	}
	if cur == ExitBlocked {
		return cur
	}
	if next == ExitInsufficient {
		return ExitInsufficient
	}
	return cur
}

func dqRunRemote(ctx context.Context, client *APIClient, env, contractID string, yamlBody []byte, failOn string) (dqcontract.RunReport, int) {
	body := map[string]any{"fail_on": failOn}
	if len(yamlBody) > 0 {
		body["yaml"] = string(yamlBody)
		body["adhoc"] = true
	}
	payload, _ := json.Marshal(body)
	path := fmt.Sprintf("/api/v1/environments/%s/quality-contracts/runs", env)
	if contractID != "" {
		path = fmt.Sprintf("/api/v1/environments/%s/quality-contracts/%s/runs", env, contractID)
	}
	resp, err := client.Do(ctx, http.MethodPost, path, bytes.NewReader(payload))
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return dqcontract.RunReport{}, ExitInsufficient
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusConflict {
		fmt.Fprintln(os.Stderr, "recurso de qualidade desabilitado")
		return dqcontract.RunReport{}, ExitUsage
	}
	if resp.StatusCode == http.StatusBadRequest {
		fmt.Fprintln(os.Stderr, strings.TrimSpace(string(raw)))
		return dqcontract.RunReport{}, ExitUsage
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		fmt.Fprintln(os.Stderr, "credencial ou ambiente não autorizado")
		return dqcontract.RunReport{}, ExitUsage
	}
	if resp.StatusCode >= 300 {
		fmt.Fprintln(os.Stderr, config.SanitizeError(fmt.Errorf("API %d: %s", resp.StatusCode, string(raw))))
		return dqcontract.RunReport{}, MapHTTPStatusToExit(resp.StatusCode)
	}
	var rep dqcontract.RunReport
	if err := json.Unmarshal(raw, &rep); err != nil {
		var wrap struct {
			Report dqcontract.RunReport `json:"report"`
		}
		if err2 := json.Unmarshal(raw, &wrap); err2 != nil {
			fmt.Fprintln(os.Stderr, "resposta inválida da API")
			return dqcontract.RunReport{}, ExitInsufficient
		}
		rep = wrap.Report
	}
	if rep.SchemaVersion == "" {
		rep.SchemaVersion = "1"
	}
	switch dqcontract.GateExit(rep.Results, failOn) {
	case 1:
		return rep, ExitBlocked
	case 3:
		return rep, ExitInsufficient
	default:
		return rep, ExitOK
	}
}

func dqCollectYAMLFiles(file, dir string) ([]string, error) {
	var out []string
	if file != "" {
		out = append(out, file)
	}
	if dir != "" {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			lower := strings.ToLower(name)
			if strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml") {
				out = append(out, filepath.Join(dir, name))
			}
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("nenhum YAML encontrado")
	}
	return out, nil
}

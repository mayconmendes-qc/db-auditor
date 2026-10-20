package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/osmendes/db-auditor/internal/collectors/postgres"
	"github.com/osmendes/db-auditor/internal/config"
	"github.com/osmendes/db-auditor/internal/migrationrisk"
)

func runMigrationRisk(args []string) int {
	fs := flag.NewFlagSet("migration-risk", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	pathFlag := fs.String("path", "", "arquivo .sql ou diretório (repetível via múltiplos --path após parse manual)")
	pgVersion := fs.Int("pg-version", 0, "major do PostgreSQL alvo (14–18)")
	failOn := fs.String("fail-on", "high", "severidade mínima para código 1")
	format := fs.String("format", "md", "md ou json")
	envID := fs.String("environment", "", "UUID do ambiente (modo snapshot via API)")
	apiURL := fs.String("api-url", "", "URL base da API (ou AUDITOR_API_URL)")
	direct := fs.Bool("direct", false, "modo local: DSN em AUDITOR_MIGRATION_RISK_DSN/_FILE")
	submit := fs.Bool("submit", false, "envia o resultado ao auditor (escopo migration-analysis:write)")
	repo := fs.String("repository", "", "repositório (para --submit)")
	pr := fs.String("pr", "", "número do PR (para --submit)")
	commit := fs.String("commit", "", "commit SHA (para --submit)")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	if err := RefuseSecretFlags(args); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return ExitUsage
	}
	paths := collectPaths(*pathFlag, fs.Args())
	// Also accept repeated --path by rescanning raw args.
	paths = append(paths, extractRepeatPaths(args)...)
	paths = uniqueStrings(paths)
	if len(paths) == 0 {
		fmt.Fprintln(os.Stderr, "informe --path <arquivo|diretório>")
		return ExitUsage
	}
	fmtName, err := FormatFlag(*format)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return ExitUsage
	}
	th := migrationrisk.DefaultThresholds()
	th.FailOn = strings.ToLower(strings.TrimSpace(*failOn))
	ctx := migrationrisk.Context{Mode: "none", PGVersion: *pgVersion}
	if *direct {
		code, c := loadDirectContext(*pgVersion)
		if code != ExitOK {
			return code
		}
		ctx = c
	} else if *envID != "" {
		code, c := loadSnapshotContext(*apiURL, *envID, *pgVersion, paths)
		if code != ExitOK {
			return code
		}
		ctx = c
	} else if *pgVersion == 0 {
		ctx.PGVersion = 16
	}
	rep, err := migrationrisk.AnalyzePaths(paths, ctx, th)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "supressão") {
			fmt.Fprintln(os.Stderr, msg)
			return ExitUsage
		}
		fmt.Fprintln(os.Stderr, config.SanitizeError(err))
		return ExitInsufficient
	}
	exit := migrationrisk.GateExit(rep, th.FailOn, nil)
	if *submit && *envID != "" {
		if code := submitAnalysis(*apiURL, *envID, *repo, *pr, *commit, rep); code != ExitOK {
			return code
		}
	}
	if fmtName == "json" {
		if err := WriteJSON(os.Stdout, rep); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return ExitUsage
		}
	} else {
		if err := WriteMarkdown(os.Stdout, migrationrisk.RenderMarkdown(rep)); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return ExitUsage
		}
	}
	return exit
}

func collectPaths(primary string, rest []string) []string {
	var out []string
	if primary != "" {
		out = append(out, primary)
	}
	out = append(out, rest...)
	return out
}

func extractRepeatPaths(args []string) []string {
	var out []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--path" && i+1 < len(args) {
			out = append(out, args[i+1])
			i++
			continue
		}
		if strings.HasPrefix(a, "--path=") {
			out = append(out, strings.TrimPrefix(a, "--path="))
		}
	}
	return out
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func loadDirectContext(pgVersion int) (int, migrationrisk.Context) {
	dsn, err := ReadSecret("AUDITOR_MIGRATION_RISK_DSN", "AUDITOR_MIGRATION_RISK_DSN_FILE")
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return ExitUsage, migrationrisk.Context{}
	}
	if strings.TrimSpace(dsn) == "" {
		fmt.Fprintln(os.Stderr, "modo --direct exige AUDITOR_MIGRATION_RISK_DSN ou AUDITOR_MIGRATION_RISK_DSN_FILE")
		return ExitUsage, migrationrisk.Context{}
	}
	if err := config.ValidateTargetDSNs(map[string]string{"cli": dsn}); err != nil {
		fmt.Fprintln(os.Stderr, "DSN recusado pela política de segurança do alvo")
		return ExitSecurityRefused, migrationrisk.Context{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := postgres.ConnectReadOnly(ctx, dsn)
	if err != nil {
		msg := config.SanitizeError(err)
		fmt.Fprintln(os.Stderr, msg)
		if strings.Contains(err.Error(), "write or administration privilege") ||
			strings.Contains(err.Error(), "not read-only") {
			return ExitSecurityRefused, migrationrisk.Context{}
		}
		return ExitInsufficient, migrationrisk.Context{}
	}
	defer conn.Close(context.Background())
	c, err := migrationrisk.LoadDirectContext(ctx, conn, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, config.SanitizeError(err))
		return ExitInsufficient, migrationrisk.Context{}
	}
	if pgVersion > 0 {
		c.PGVersion = pgVersion
	}
	return ExitOK, c
}

func loadSnapshotContext(apiURL, envID string, pgVersion int, paths []string) (int, migrationrisk.Context) {
	token, err := RequireCLIToken()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return ExitUsage, migrationrisk.Context{}
	}
	base := apiURL
	if base == "" {
		base = os.Getenv("AUDITOR_API_URL")
	}
	if base == "" {
		fmt.Fprintln(os.Stderr, "informe --api-url ou AUDITOR_API_URL")
		return ExitUsage, migrationrisk.Context{}
	}
	// Discover tables from a quick static pass without rules.
	rep, err := migrationrisk.AnalyzePaths(paths, migrationrisk.Context{Mode: "none", PGVersion: 16}, migrationrisk.DefaultThresholds())
	tables := map[string]bool{}
	if err == nil {
		for _, f := range rep.Findings {
			if f.Object != "" {
				tables[f.Object] = true
			}
		}
	}
	list := make([]string, 0, len(tables))
	for t := range tables {
		list = append(list, t)
	}
	client := NewAPIClient(base, token, 30*time.Second)
	q := url.Values{}
	if len(list) > 0 {
		q.Set("tables", strings.Join(list, ","))
	}
	path := "/api/v1/environments/" + envID + "/migration-context"
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}
	resp, err := client.Do(context.Background(), http.MethodGet, path, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return ExitInsufficient, migrationrisk.Context{}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "contexto indisponível (HTTP %d)\n", resp.StatusCode)
		return MapHTTPStatusToExit(resp.StatusCode), migrationrisk.Context{}
	}
	var payload struct {
		Partial      bool   `json:"partial"`
		Stale        bool   `json:"stale"`
		Missing      bool   `json:"missing"`
		AuditRunID   string `json:"audit_run_id"`
		AuditRunAt   string `json:"audit_run_at"`
		ServerVerNum int    `json:"server_version_num"`
		Tables       []struct {
			Name         string  `json:"name"`
			SizeBytes    int64   `json:"size_bytes"`
			RowEstimate  int64   `json:"row_estimate"`
			WriteRate    float64 `json:"write_rate_per_s"`
			HasWriteRate bool    `json:"has_write_rate"`
		} `json:"tables"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		fmt.Fprintln(os.Stderr, "resposta de contexto inválida")
		return ExitInsufficient, migrationrisk.Context{}
	}
	c := migrationrisk.Context{
		Mode:       "snapshot",
		Partial:    payload.Partial || payload.Stale || payload.Missing,
		Stale:      payload.Stale,
		Missing:    payload.Missing,
		AuditRunID: payload.AuditRunID,
		AuditRunAt: payload.AuditRunAt,
		Tables:     map[string]migrationrisk.TableContext{},
	}
	if payload.ServerVerNum > 0 {
		c.PGVersion = payload.ServerVerNum / 10000
	}
	if pgVersion > 0 {
		c.PGVersion = pgVersion
	}
	if c.PGVersion == 0 {
		c.PGVersion = 16
	}
	for _, t := range payload.Tables {
		key := strings.ToLower(t.Name)
		c.Tables[key] = migrationrisk.TableContext{
			Qualified:    key,
			SizeBytes:    t.SizeBytes,
			RowEstimate:  t.RowEstimate,
			WriteRate:    t.WriteRate,
			HasWriteRate: t.HasWriteRate,
		}
	}
	return ExitOK, c
}

func submitAnalysis(apiURL, envID, repo, pr, commit string, rep migrationrisk.Report) int {
	token, err := RequireCLIToken()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return ExitUsage
	}
	base := apiURL
	if base == "" {
		base = os.Getenv("AUDITOR_API_URL")
	}
	if base == "" {
		fmt.Fprintln(os.Stderr, "informe --api-url ou AUDITOR_API_URL para --submit")
		return ExitUsage
	}
	items := make([]map[string]any, 0, len(rep.Findings))
	for _, f := range rep.Findings {
		item := map[string]any{
			"file": f.File, "line": f.Line, "code": f.Code,
			"operation": f.Operation, "object": f.Object,
			"lock": f.Lock, "rewrite_or_scan": f.RewriteScan,
			"severity": f.Severity, "confidence": f.Confidence,
			"suppressed": f.Suppressed, "justification": truncate(f.Justification, 500),
		}
		if f.SizeBytes != nil {
			item["size_bytes"] = *f.SizeBytes
		}
		if f.DurationEst != "" {
			item["duration_estimate"] = f.DurationEst
		}
		items = append(items, item)
	}
	payload := map[string]any{
		"repository":       truncate(repo, 200),
		"pr":               truncate(pr, 32),
		"commit":           truncate(commit, 64),
		"highest_severity": rep.Summary.Highest,
		"rules_version":    rep.RulesVersion,
		"pg_version":       rep.PGVersion,
		"context_mode":     rep.ContextMode,
		"audit_run_id":     rep.AuditRunID,
		"items":            items,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ExitUsage
	}
	client := NewAPIClient(base, token, 30*time.Second)
	resp, err := client.Do(context.Background(), http.MethodPost,
		"/api/v1/environments/"+envID+"/migration-analyses",
		strings.NewReader(string(raw)))
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return ExitInsufficient
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "falha ao enviar análise (HTTP %d)\n", resp.StatusCode)
		return MapHTTPStatusToExit(resp.StatusCode)
	}
	return ExitOK
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// ParsePGMajor converts server_version_num or major string.
func ParsePGMajor(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	if n > 10000 {
		return n / 10000, nil
	}
	return n, nil
}

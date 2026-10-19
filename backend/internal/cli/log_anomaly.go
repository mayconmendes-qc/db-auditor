package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/osmendes/db-auditor/internal/config"
	"github.com/osmendes/db-auditor/internal/loganomaly"
)

func runLogAnomaly(args []string) int {
	fs := flag.NewFlagSet("log-anomaly", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	profile := fs.String("profile", loganomaly.ProfilePostgresJSONLog, "postgres-jsonlog ou generico")
	pathFlag := fs.String("path", "", "arquivo ou diretório de logs JSON (omitir = stdin)")
	timeField := fs.String("time-field", "", "campo de horário (perfil generico)")
	levelField := fs.String("level-field", "level", "campo de nível (perfil generico)")
	messageField := fs.String("message-field", "", "campo de mensagem (perfil generico)")
	window := fs.Duration("window", 5*time.Minute, "tamanho da janela")
	baselineK := fs.Int("baseline-windows", 12, "K janelas na linha de base")
	failOn := fs.String("fail-on", "high", "severidade mínima para código 1")
	format := fs.String("format", "md", "md ou json")
	lastLoginOut := fs.String("last-login-out", "", "arquivo local de último login por role (só postgres-jsonlog); use - para incluir na saída")
	submitLastLogin := fs.Bool("submit-last-login", false, "envia só role+horário ao ambiente (escopo last-login:write)")
	environment := fs.String("environment", "", "UUID do ambiente (obrigatório com --submit-last-login)")
	apiURL := fs.String("api-url", "", "URL base da API (ou AUDITOR_API_URL)")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	if err := RefuseSecretFlags(args); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return ExitUsage
	}

	paths := collectPaths(*pathFlag, nil)
	paths = append(paths, extractRepeatPaths(args)...)
	paths = uniqueStrings(paths)

	fmtName, err := FormatFlag(*format)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return ExitUsage
	}

	submit := *submitLastLogin
	envID := strings.TrimSpace(*environment)
	if submit && envID == "" {
		fmt.Fprintln(os.Stderr, "--submit-last-login exige --environment <uuid>")
		return ExitUsage
	}

	opts := loganomaly.Options{
		Profile:          strings.TrimSpace(*profile),
		Paths:            paths,
		TimeField:        *timeField,
		LevelField:       *levelField,
		MessageField:     *messageField,
		Window:           *window,
		BaselineK:        *baselineK,
		FailOn:           strings.ToLower(strings.TrimSpace(*failOn)),
		Format:           fmtName,
		LastLoginOut:     strings.TrimSpace(*lastLoginOut),
		CollectLastLogin: submit || strings.TrimSpace(*lastLoginOut) != "",
		Thresholds:       loganomaly.DefaultThresholds(),
	}

	var stdin = os.Stdin
	if len(paths) > 0 {
		stdin = nil
	}

	res, err := loganomaly.Analyze(opts, stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, config.SanitizeError(err))
		if res.ExitCode != 0 {
			return res.ExitCode
		}
		return ExitUsage
	}

	// Local output first so a failed submit still preserves the report (plan 10 L5).
	if fmtName == "json" {
		if err := WriteJSON(os.Stdout, res.Report); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return ExitUsage
		}
	} else {
		if err := WriteMarkdown(os.Stdout, loganomaly.RenderMarkdown(res.Report)); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return ExitUsage
		}
	}

	if submit {
		if res.Report.LastLogin == nil {
			fmt.Fprintln(os.Stderr, "nenhum agregado de último login para enviar")
			return ExitUsage
		}
		token, err := RequireCLIToken()
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return ExitUsage
		}
		base := strings.TrimSpace(*apiURL)
		if base == "" {
			base = strings.TrimSpace(os.Getenv("AUDITOR_API_URL"))
		}
		if base == "" {
			fmt.Fprintln(os.Stderr, "informe --api-url ou AUDITOR_API_URL para --submit-last-login")
			return ExitUsage
		}
		client := NewAPIClient(base, token, 30*time.Second)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := loganomaly.SubmitLastLogin(ctx, client, envID, *res.Report.LastLogin); err != nil {
			fmt.Fprintln(os.Stderr, "envio do último login: "+config.SanitizeError(err))
			return ExitUsage
		}
	}

	return res.ExitCode
}

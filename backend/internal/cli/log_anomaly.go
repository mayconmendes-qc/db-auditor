package cli

import (
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

	opts := loganomaly.Options{
		Profile:      strings.TrimSpace(*profile),
		Paths:        paths,
		TimeField:    *timeField,
		LevelField:   *levelField,
		MessageField: *messageField,
		Window:       *window,
		BaselineK:    *baselineK,
		FailOn:       strings.ToLower(strings.TrimSpace(*failOn)),
		Format:       fmtName,
		LastLoginOut: strings.TrimSpace(*lastLoginOut),
		Thresholds:   loganomaly.DefaultThresholds(),
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
	return res.ExitCode
}

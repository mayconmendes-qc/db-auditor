package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/osmendes/db-auditor/internal/buildinfo"
	"github.com/osmendes/db-auditor/internal/config"
	"github.com/osmendes/db-auditor/internal/restoreverify"
)

func runRestoreVerify(args []string) int {
	fs := flag.NewFlagSet("restore-verify", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	backup := fs.String("backup", "", "arquivo ou diretório pg_dump (custom/directory)")
	sha := fs.String("sha256", "", "checksum sha256 esperado do artefato (opcional)")
	envID := fs.String("environment", "", "UUID do ambiente")
	runtime := fs.String("runtime", "auto", "docker, podman ou auto")
	image := fs.String("image", "", "imagem PostgreSQL (major ≥ origem), ex. postgres:16")
	confirm := fs.Bool("confirm-isolated-target", false, "confirma destino isolado (obrigatório)")
	submit := fs.Bool("submit", false, "envia evidência target_restore_drill à API")
	sampleHash := fs.Bool("sample-hash", false, "opt-in: hash de amostra (exige DSN só leitura no runner)")
	apiURL := fs.String("api-url", "", "URL base da API (ou AUDITOR_API_URL)")
	format := fs.String("format", "md", "md ou json")
	timeout := fs.Duration("timeout", restoreverify.DefaultTimeout, "teto de execução")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	if err := RefuseSecretFlags(args); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return ExitUsage
	}

	fmtName, err := FormatFlag(*format)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return ExitUsage
	}

	sourceDSN := ""
	if *sampleHash {
		sourceDSN, err = ReadSecret("AUDITOR_RESTORE_VERIFY_SOURCE_DSN", "AUDITOR_RESTORE_VERIFY_SOURCE_DSN_FILE")
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return ExitUsage
		}
		if strings.TrimSpace(sourceDSN) == "" {
			fmt.Fprintln(os.Stderr, "--sample-hash exige AUDITOR_RESTORE_VERIFY_SOURCE_DSN ou _FILE")
			return ExitUsage
		}
	}

	opt := restoreverify.Options{
		BackupPath:      strings.TrimSpace(*backup),
		ExpectedSHA256:  strings.ToLower(strings.TrimSpace(*sha)),
		EnvironmentID:   strings.TrimSpace(*envID),
		Runtime:         strings.TrimSpace(*runtime),
		Image:           strings.TrimSpace(*image),
		ConfirmIsolated: *confirm,
		Submit:          *submit,
		SampleHash:      *sampleHash,
		SourceDSN:       sourceDSN,
		CLIVersion:      buildinfo.String(),
		Timeout:         *timeout,
	}
	if opt.BackupPath == "" || opt.EnvironmentID == "" || opt.Image == "" {
		fmt.Fprintln(os.Stderr, "informe --backup, --environment e --image")
		return ExitUsage
	}

	ctx, cancel := context.WithTimeout(context.Background(), opt.Timeout+time.Minute)
	defer cancel()

	baseURL := strings.TrimSpace(*apiURL)
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv("AUDITOR_API_URL"))
	}
	opt.APIURL = baseURL

	token := ""
	if *submit || baseURL != "" {
		token, err = RequireCLIToken()
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return ExitUsage
		}
		opt.CLIToken = token
	}

	if baseURL != "" && token != "" {
		dc, err := restoreverify.FetchDrillContext(ctx, baseURL, token, opt.EnvironmentID, nil)
		if err != nil {
			fmt.Fprintln(os.Stderr, config.SanitizeError(err))
			return ExitInsufficient
		}
		opt.SourceMajor = dc.SourceMajor
		opt.TargetFingerprints = dc.Fingerprints
		opt.Inventory = dc.Inventory
		opt.EstimatedRows = dc.EstimatedRows
		opt.WriteCounters = dc.WriteCounters
	}

	res := restoreverify.Run(ctx, opt)
	if *submit && token != "" && baseURL != "" {
		if err := restoreverify.SubmitEvidence(ctx, baseURL, token, opt.EnvironmentID, res.Evidence, nil); err != nil {
			fmt.Fprintln(os.Stderr, "envio da evidência: "+config.SanitizeError(err))
			if res.ExitCode == ExitOK {
				return ExitInsufficient
			}
		}
	}

	if fmtName == "json" {
		if err := WriteJSON(os.Stdout, res.Evidence); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return ExitUsage
		}
	} else {
		_ = WriteMarkdown(os.Stdout, res.Summary+"\n")
		if res.Evidence.Note != "" {
			_, _ = fmt.Fprintln(os.Stderr, res.Evidence.Note)
		}
	}
	return res.ExitCode
}

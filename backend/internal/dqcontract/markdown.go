package dqcontract

import (
	"fmt"
	"strings"
	"time"
)

// RunReport is the Markdown/JSON-facing summary of a contract run.
type RunReport struct {
	SchemaVersion string        `json:"schema_version"`
	EnvironmentID string        `json:"environment_id"`
	ContractID    string        `json:"contract_id,omitempty"`
	VersionID     string        `json:"version_id,omitempty"`
	Mode          string        `json:"mode"`
	Status        string        `json:"status"`
	Coverage      string        `json:"coverage"`
	Database      string        `json:"database"`
	Schema        string        `json:"schema"`
	Table         string        `json:"table"`
	SampleLimit   int           `json:"sample_limit"`
	StartedAt     time.Time     `json:"started_at"`
	FinishedAt    time.Time     `json:"finished_at"`
	Results       []CheckResult `json:"results"`
	SamplingNote  string        `json:"sampling_note"`
}

// RenderMarkdown builds the RF-09 Markdown table.
func RenderMarkdown(rep RunReport) string {
	var b strings.Builder
	b.WriteString("# Contratos de qualidade\n\n")
	fmt.Fprintf(&b, "- Ambiente: `%s`\n", rep.EnvironmentID)
	fmt.Fprintf(&b, "- Tabela: `%s.%s.%s`\n", rep.Database, rep.Schema, rep.Table)
	fmt.Fprintf(&b, "- Modo: %s · estado: %s · cobertura: %s\n", rep.Mode, rep.Status, rep.Coverage)
	fmt.Fprintf(&b, "- Amostra (contrato): %d\n", rep.SampleLimit)
	if !rep.StartedAt.IsZero() {
		fmt.Fprintf(&b, "- Início: %s\n", rep.StartedAt.UTC().Format(time.RFC3339))
	}
	b.WriteString("\n| Checagem | Resultado | Severidade | Contagem / %% | Amostra | Motivo |\n")
	b.WriteString("|---|---|---|---|---|---|\n")
	for _, r := range rep.Results {
		count := "—"
		if r.ObservedPct != nil {
			count = fmt.Sprintf("%.2f%%", *r.ObservedPct)
			if r.ObservedCount != nil {
				count = fmt.Sprintf("%d (%.2f%%)", *r.ObservedCount, *r.ObservedPct)
			}
		} else if r.ObservedCount != nil {
			count = fmt.Sprintf("%d", *r.ObservedCount)
		} else if r.FreshnessAgeHours != nil {
			count = fmt.Sprintf("%.0f h", *r.FreshnessAgeHours)
		}
		sample := "—"
		if r.SampleSize != nil {
			sample = fmt.Sprintf("%d (%s)", *r.SampleSize, r.SampleMethod)
		} else if r.SampleMethod != "" {
			sample = r.SampleMethod
		}
		label := r.CheckKey
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n",
			escapeMD(label), escapeMD(r.Result), escapeMD(r.Severity),
			escapeMD(count), escapeMD(sample), escapeMD(r.Reason))
	}
	b.WriteString("\n")
	note := rep.SamplingNote
	if note == "" {
		note = SamplingNote
	}
	b.WriteString(note)
	b.WriteString("\n")
	return b.String()
}

func escapeMD(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "|", "\\|"), "\n", " ")
}

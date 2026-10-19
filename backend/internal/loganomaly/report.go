package loganomaly

import (
	"fmt"
	"strings"
	"time"
)

// Report is the schema_versioned analysis payload (RF-06).
type Report struct {
	SchemaVersion    string          `json:"schema_version"`
	Profile          string          `json:"profile"`
	WindowSeconds    int             `json:"window_seconds"`
	Stats            ReadStats       `json:"stats"`
	Templates        int             `json:"templates"`
	TemplateOverflow int64           `json:"template_overflow,omitempty"`
	Baseline         BaselineSummary `json:"baseline"`
	PeriodStart      string          `json:"period_start,omitempty"`
	PeriodEnd        string          `json:"period_end,omitempty"`
	Anomalies        []Anomaly       `json:"anomalies"`
	Summary          Summary         `json:"summary"`
	Caveats          []string        `json:"caveats,omitempty"`
	LastLogin        *LastLoginFile  `json:"last_login,omitempty"` // optional section; never has IP/db
}

// BaselineSummary is a JSON-safe baseline overview.
type BaselineSummary struct {
	Windows      int    `json:"windows"`
	Insufficient bool   `json:"insufficient"`
	From         string `json:"from,omitempty"`
	To           string `json:"to,omitempty"`
}

// Summary counts anomalies by severity.
type Summary struct {
	Info     int    `json:"info"`
	Low      int    `json:"low"`
	Medium   int    `json:"medium"`
	High     int    `json:"high"`
	Critical int    `json:"critical"`
	Highest  string `json:"highest"`
	Total    int    `json:"total"`
}

// BuildSummary aggregates anomaly severities.
func BuildSummary(anoms []Anomaly) Summary {
	s := Summary{Highest: SeverityInfo}
	rank := map[string]int{SeverityInfo: 1, SeverityLow: 2, SeverityMedium: 3, SeverityHigh: 4, SeverityCritical: 5}
	highest := 0
	for _, a := range anoms {
		s.Total++
		switch a.Severity {
		case SeverityInfo:
			s.Info++
		case SeverityLow:
			s.Low++
		case SeverityMedium:
			s.Medium++
		case SeverityHigh:
			s.High++
		case SeverityCritical:
			s.Critical++
		}
		if r := rank[a.Severity]; r > highest {
			highest = r
			s.Highest = a.Severity
		}
	}
	if s.Total == 0 {
		s.Highest = SeverityInfo
	}
	return s
}

// RenderMarkdown formats the report for terminal/Markdown output.
func RenderMarkdown(rep Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Anomalias de log\n\n")
	fmt.Fprintf(&b, "schema_version: %s  \n", rep.SchemaVersion)
	fmt.Fprintf(&b, "perfil: %s  \n", rep.Profile)
	fmt.Fprintf(&b, "janela: %ds  \n\n", rep.WindowSeconds)

	fmt.Fprintf(&b, "## Resumo\n\n")
	fmt.Fprintf(&b, "- linhas lidas: %d (inválidas: %d, >64KB: %d, ok: %d)\n",
		rep.Stats.LinesRead, rep.Stats.LinesInvalid, rep.Stats.LinesOversized, rep.Stats.LinesParsed)
	fmt.Fprintf(&b, "- templates: %d", rep.Templates)
	if rep.TemplateOverflow > 0 {
		fmt.Fprintf(&b, " (excedente em outros: %d eventos)", rep.TemplateOverflow)
	}
	fmt.Fprintf(&b, "\n")
	if rep.PeriodStart != "" {
		fmt.Fprintf(&b, "- período: %s → %s\n", rep.PeriodStart, rep.PeriodEnd)
	}
	fmt.Fprintf(&b, "- base: %d janelas", rep.Baseline.Windows)
	if rep.Baseline.Insufficient {
		fmt.Fprintf(&b, " (**insuficiente**)")
	}
	fmt.Fprintf(&b, "\n")
	fmt.Fprintf(&b, "- anomalias: %d (alta: %d, média: %d, baixa: %d, info: %d)\n\n",
		rep.Summary.Total, rep.Summary.High+rep.Summary.Critical, rep.Summary.Medium, rep.Summary.Low, rep.Summary.Info)

	if len(rep.Caveats) > 0 {
		fmt.Fprintf(&b, "## Ressalvas\n\n")
		for _, c := range rep.Caveats {
			fmt.Fprintf(&b, "- %s\n", c)
		}
		fmt.Fprintf(&b, "\n")
	}

	fmt.Fprintf(&b, "## Anomalias\n\n")
	if len(rep.Anomalies) == 0 {
		fmt.Fprintf(&b, "_Nenhuma anomalia acima dos limiares._\n\n")
	} else {
		fmt.Fprintf(&b, "| Severidade | Tipo | Janela | Obs. | Esperado | Fator | z | Template |\n")
		fmt.Fprintf(&b, "|---|---|---|---:|---:|---:|---:|---|\n")
		for _, a := range rep.Anomalies {
			tmpl := a.Template
			if tmpl == "" {
				tmpl = a.TemplateID
			}
			if len(tmpl) > 80 {
				tmpl = tmpl[:77] + "..."
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %d | %.2f | %.2f | %.2f | `%s` |\n",
				a.Severity, a.Kind, a.WindowStart.UTC().Format(time.RFC3339),
				a.Observed, a.Expected, a.Factor, a.ZScore, tmpl)
		}
		fmt.Fprintf(&b, "\n")
	}

	if rep.LastLogin != nil && len(rep.LastLogin.Entries) > 0 {
		fmt.Fprintf(&b, "## Último login por role (local)\n\n")
		fmt.Fprintf(&b, "| Role | last_login_at |\n|---|---|\n")
		for _, e := range rep.LastLogin.Entries {
			fmt.Fprintf(&b, "| `%s` | %s |\n", e.Role, e.LastLoginAt)
		}
		fmt.Fprintf(&b, "\n")
	}

	fmt.Fprintf(&b, "_Saídas contêm só templates mascarados e contagens. Linhas brutas não são emitidas (LGPD)._\n")
	return b.String()
}

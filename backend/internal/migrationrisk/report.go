package migrationrisk

import (
	"fmt"
	"strings"
)

// RenderMarkdown builds the PR comment body (no SQL literals).
func RenderMarkdown(rep Report) string {
	var b strings.Builder
	b.WriteString("<!-- db-auditor:migration-risk -->\n")
	b.WriteString("## Risco de migration\n\n")
	fmt.Fprintf(&b, "Maior severidade: **%s**. Contagem: crítica=%d alta=%d média=%d baixa=%d info=%d.\n\n",
		rep.Summary.Highest, rep.Summary.Critical, rep.Summary.High, rep.Summary.Medium, rep.Summary.Low, rep.Summary.Info)
	if rep.Partial {
		b.WriteString("> Contexto parcial ou desatualizado — trate o resultado com cautela (código de saída 3).\n\n")
	}
	if rep.Truncated {
		b.WriteString("> Análise truncada em 500 operações.\n\n")
	}
	b.WriteString("| Arquivo:linha | Código | Operação | Objeto | Lock | Reescrita/varredura | Tamanho | Duração | Severidade | Confiança | Alternativa |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|---|---|\n")
	for _, f := range rep.Findings {
		sev := f.Severity
		if f.Suppressed {
			sev = "suprimida"
		}
		if f.Unanalyzed {
			sev = "não analisada"
		}
		size := "—"
		if f.SizeBytes != nil {
			size = formatBytes(*f.SizeBytes)
		}
		dur := f.DurationEst
		if dur == "" {
			dur = "—"
		}
		alt := f.Alternative
		if f.Suppressed && f.Justification != "" {
			alt = "suprimida: " + f.Justification
		}
		fmt.Fprintf(&b, "| %s:%d | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s |\n",
			escapeMD(f.File), f.Line, f.Code, escapeMD(f.Operation), escapeMD(f.Object),
			escapeMD(f.Lock), escapeMD(f.RewriteScan), size, escapeMD(dur), sev, f.Confidence, escapeMD(alt))
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "Modo de contexto: `%s`. Versão das regras: `%s`. PostgreSQL alvo: %d.\n",
		rep.ContextMode, rep.RulesVersion, rep.PGVersion)
	if rep.AuditRunID != "" {
		fmt.Fprintf(&b, "Execução de referência: `%s`", rep.AuditRunID)
		if rep.AuditRunAt != "" {
			fmt.Fprintf(&b, " (%s)", rep.AuditRunAt)
		}
		b.WriteString(".\n")
	}
	fmt.Fprintf(&b, "schema_version: %s\n", SchemaVersion)
	return b.String()
}

func escapeMD(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

func formatBytes(n int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case n >= gb:
		return fmt.Sprintf("%.1f GB", float64(n)/float64(gb))
	case n >= mb:
		return fmt.Sprintf("%.1f MB", float64(n)/float64(mb))
	case n >= kb:
		return fmt.Sprintf("%.1f KB", float64(n)/float64(kb))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

package digest

import (
	"fmt"
	"strings"
	"time"
)

const maxMarkdownBytes = 100 * 1024

// MarkdownInput is the sanitized content for the digest body (no SQL literals).
type MarkdownInput struct {
	EnvironmentName string
	EnvironmentID   string
	State           string
	Reason          string
	PriorFrom       time.Time
	PriorTo         time.Time
	CurrentFrom     time.Time
	CurrentTo       time.Time
	PriorFirstRun   string
	PriorLastRun    string
	CurrentFirstRun string
	CurrentLastRun  string
	WorkloadLimitN  int
	SubwindowNote   string
	Sections        DigestSections
	CoverageNotes   []string
}

// RenderMarkdown builds the e-mail/UI Markdown body. Never includes SQL text.
func RenderMarkdown(in MarkdownInput) string {
	var b strings.Builder
	b.WriteString("# Digest de consultas\n\n")
	fmt.Fprintf(&b, "- **Ambiente:** %s (`%s`)\n", sanitizeText(in.EnvironmentName), in.EnvironmentID)
	fmt.Fprintf(&b, "- **Estado:** %s\n", in.State)
	if in.Reason != "" {
		fmt.Fprintf(&b, "- **Motivo:** %s\n", sanitizeText(in.Reason))
	}
	if in.State == "ok" {
		b.WriteString(fmt.Sprintf("- **Janela anterior:** %s → %s (coletas `%s` … `%s`)\n",
			fmtTime(in.PriorFrom), fmtTime(in.PriorTo), shortID(in.PriorFirstRun), shortID(in.PriorLastRun)))
		b.WriteString(fmt.Sprintf("- **Janela atual:** %s → %s (coletas `%s` … `%s`)\n",
			fmtTime(in.CurrentFrom), fmtTime(in.CurrentTo), shortID(in.CurrentFirstRun), shortID(in.CurrentLastRun)))
		if in.SubwindowNote != "" {
			b.WriteString(fmt.Sprintf("- **Subjanela:** %s\n", sanitizeText(in.SubwindowNote)))
		}
		b.WriteString(fmt.Sprintf("- **Top N coletado:** %d\n", in.WorkloadLimitN))
	}
	b.WriteString("\n")

	if in.State != "ok" {
		b.WriteString("Não há seções de classificação para este digest.\n")
		return truncateMarkdown(b.String())
	}

	writeSection(&b, "Regressões", in.Sections.Regressions, true)
	writeSection(&b, "Consultas novas", in.Sections.NewQueries, true)
	writeSection(&b, "Consultas que sumiram", in.Sections.GoneQueries, false)
	writeSection(&b, "Melhoras", in.Sections.Improvements, true)

	b.WriteString("## Ressalvas\n\n")
	b.WriteString(fmt.Sprintf("- O coletor guarda só o top %d por tempo total; uma consulta \"nova\" pode apenas ter entrado no top N.\n", in.WorkloadLimitN))
	b.WriteString("- Fingerprints sem literais; o texto original de `pg_stat_statements` nunca entra no digest.\n")
	b.WriteString("- Uma refatoração de SQL pode mudar o fingerprint e aparecer como \"sumiu\" + \"nova\".\n")
	for _, note := range in.CoverageNotes {
		b.WriteString("- ")
		b.WriteString(sanitizeText(note))
		b.WriteString("\n")
	}
	return truncateMarkdown(b.String())
}

func writeSection(b *strings.Builder, title string, items []ClassifiedItem, showChange bool) {
	b.WriteString("## ")
	b.WriteString(title)
	b.WriteString("\n\n")
	if len(items) == 0 {
		b.WriteString("_Nenhum item._\n\n")
		return
	}
	b.WriteString("| Fingerprint | Objetos | Chamadas (ant→atu) | Média ms (ant→atu) | Variação | Severidade |\n")
	b.WriteString("|---|---|---|---|---|---|\n")
	for _, it := range items {
		change := "—"
		if showChange && (it.Classification == ClassRegression || it.Classification == ClassImprovement || it.Classification == ClassNew) {
			change = fmt.Sprintf("%+.0f%%", it.MeanChangePct*100)
			if it.Classification == ClassNew {
				change = fmt.Sprintf("%.1f%% do total", it.CurrentShare*100)
			}
		}
		if it.Classification == ClassGone {
			change = fmt.Sprintf("%.1f%% do total anterior", it.PriorShare*100)
		}
		fmt.Fprintf(b, "| `%s` | %s | %d→%d | %.1f→%.1f | %s | %s |\n",
			shortFP(it.Fingerprint),
			strings.Join(it.ReferencedObjects, ", "),
			it.PriorCalls, it.CurrentCalls,
			it.PriorMeanMS, it.CurrentMeanMS,
			change, it.Severity,
		)
	}
	b.WriteString("\n")
}

func truncateMarkdown(s string) string {
	if len(s) <= maxMarkdownBytes {
		return s
	}
	cut := maxMarkdownBytes - 80
	if cut < 0 {
		cut = 0
	}
	return s[:cut] + "\n\n… truncado: corpo excedeu 100 KB.\n"
}

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.UTC().Format("2006-01-02 15:04 UTC")
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func shortFP(fp string) string {
	if len(fp) > 48 {
		return fp[:45] + "…"
	}
	return fp
}

// sanitizeText strips characters that could smuggle raw SQL-looking content
// into Markdown while keeping fingerprints and object names readable.
func sanitizeText(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return s
}

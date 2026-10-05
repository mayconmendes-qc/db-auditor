package report

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Document is the renderer-independent, run-scoped report representation.
// Every collection is bounded at the database layer and sorted here so the
// same run, version and filters always produce the same PDF bytes.
type Document struct {
	Type            string
	Environment     string
	RunID           string
	RunStarted      time.Time
	RunStatus       string
	ServiceVersion  string
	RuleVersion     string
	RequestedBy     string
	DatabaseFilter  string
	SchemaFilter    string
	TableFilter     string
	SeverityFilter  string
	Coverage        string
	CoverageNotes   []string
	Databases       []Database
	Tables          []Table
	Findings        []Finding
	Growth          []GrowthPoint
	Score           *int
	ScoreConfidence float64
	ScoreCategories []ScoreCategory
	TotalTables     int
	TotalFindings   int
	Baseline        *Baseline
	Truncated       bool
}

type Database struct {
	Name      string
	Schemas   int
	Tables    int
	SizeBytes int64
}
type Table struct {
	Database      string
	Schema        string
	Name          string
	SizeBytes     int64
	Rows          int64
	HasPrimaryKey bool
}
type Finding struct {
	Severity       string
	Category       string
	Database       string
	Schema         string
	Object         string
	Title          string
	Summary        string
	Recommendation string
	Confidence     float64
	Evidence       string
	RuleVersion    string
}
type Baseline struct {
	RunID         string
	Status        string
	AddedTables   int
	RemovedTables int
	ChangedTables int
}

type GrowthPoint struct {
	RunID     string
	SizeBytes int64
	Tables    int
}
type ScoreCategory struct {
	Category string
	Score    int
	Penalty  int
	Findings int
}

type Line struct {
	Text  string
	Style string
}

func BuildLines(d Document) []Line {
	if d.TotalTables == 0 {
		d.TotalTables = len(d.Tables)
	}
	if d.TotalFindings == 0 {
		d.TotalFindings = len(d.Findings)
	}
	sort.Slice(d.Databases, func(i, j int) bool { return d.Databases[i].Name < d.Databases[j].Name })
	sort.Slice(d.Tables, func(i, j int) bool {
		a, b := d.Tables[i], d.Tables[j]
		return a.Database+"/"+a.Schema+"/"+a.Name < b.Database+"/"+b.Schema+"/"+b.Name
	})
	sort.Slice(d.Findings, func(i, j int) bool {
		a, b := d.Findings[i], d.Findings[j]
		if severityRank(a.Severity) != severityRank(b.Severity) {
			return severityRank(a.Severity) < severityRank(b.Severity)
		}
		return a.Severity+a.Category+a.Database+a.Schema+a.Object+a.Title < b.Severity+b.Category+b.Database+b.Schema+b.Object+b.Title
	})
	lines := []Line{{"DB Auditor - Relatório " + reportTypeLabel(d.Type), "title"}, {"Ambiente: " + d.Environment, "body"}, {"Execução: " + d.RunID, "body"}, {"Início: " + d.RunStarted.UTC().Format(time.RFC3339) + " | Estado: " + d.RunStatus, "body"}, {"Versão do serviço: " + d.ServiceVersion + " | Regras: " + d.RuleVersion, "body"}, {"Solicitante: " + d.RequestedBy, "body"}, {"Escopo: " + scope(d), "body"}, {"", "body"}, {"Resumo executivo", "heading"}}
	critical, high := 0, 0
	severityCounts := map[string]int{}
	for _, f := range d.Findings {
		severityCounts[strings.ToLower(f.Severity)]++
		if f.Severity == "critical" {
			critical++
		}
		if f.Severity == "high" {
			high++
		}
	}
	lines = append(lines, Line{fmt.Sprintf("%d bancos | %d tabelas | %d %s (%d %s, %d %s)", len(d.Databases), d.TotalTables, d.TotalFindings, plural(d.TotalFindings, "achado", "achados"), critical, plural(critical, "crítico", "críticos"), high, plural(high, "alto", "altos")), "body"})
	lines = append(lines, Line{Text: "Achados por severidade (itens incluídos no PDF)", Style: "subheading"})
	for _, severity := range []struct{ key, label string }{{"critical", "Críticos"}, {"high", "Altos"}, {"medium", "Médios"}, {"low", "Baixos"}, {"info", "Informativos"}} {
		lines = append(lines, Line{Text: fmt.Sprintf("%s|%d|%d", severity.label, severityCounts[severity.key], len(d.Findings)), Style: "bar"})
	}
	lines = append(lines, Line{"Cobertura: " + coverageLabel(d.Coverage), "body"})
	if d.Score != nil {
		lines = append(lines, Line{fmt.Sprintf("Índice: %d/100 | confiança %.0f%%", *d.Score, d.ScoreConfidence*100), "body"})
	} else {
		lines = append(lines, Line{fmt.Sprintf("Índice indisponível | confiança %.0f%%", d.ScoreConfidence*100), "body"})
	}
	for _, c := range d.ScoreCategories {
		lines = append(lines, Line{fmt.Sprintf("%s: %d/100 (%d findings, penalidade %d)", c.Category, c.Score, c.Findings, c.Penalty), "body"})
	}
	for _, note := range d.CoverageNotes {
		lines = append(lines, Line{"Limite: " + note, "body"})
	}
	if d.Truncated {
		lines = append(lines, Line{"Limite: inventario truncado para manter memoria previsivel.", "body"})
	}
	if d.Baseline != nil {
		lines = append(lines, Line{"Historico e baseline", "heading"}, Line{fmt.Sprintf("Baseline %s | %s | +%d / -%d tabelas, %d alteradas", d.Baseline.RunID, d.Baseline.Status, d.Baseline.AddedTables, d.Baseline.RemovedTables, d.Baseline.ChangedTables), "body"})
	}
	if len(d.Growth) > 0 {
		lines = append(lines, Line{"Evolucao de armazenamento", "heading"})
		for _, point := range d.Growth {
			lines = append(lines, Line{fmt.Sprintf("Execucao %s: %d bytes / %d tabelas", point.RunID, point.SizeBytes, point.Tables), "body"})
		}
	}
	if d.Type == "technical" || d.Type == "table" {
		lines = append(lines, Line{"Inventario tecnico", "heading"})
		for _, db := range d.Databases {
			lines = append(lines, Line{fmt.Sprintf("Banco %s: %d schemas, %d tabelas, %d bytes", db.Name, db.Schemas, db.Tables, db.SizeBytes), "body"})
		}
		for _, table := range d.Tables {
			primaryKey := "Não"
			if table.HasPrimaryKey {
				primaryKey = "Sim"
			}
			lines = append(lines, Line{fmt.Sprintf("%s.%s.%s | %d bytes | %d linhas estimadas | PK: %s", table.Database, table.Schema, table.Name, table.SizeBytes, table.Rows, primaryKey), "body"})
		}
	}
	lines = append(lines, Line{"Riscos e prioridades", "heading"})
	if len(d.Findings) == 0 {
		lines = append(lines, Line{"Nenhum achado observado neste escopo da execução.", "body"})
	}
	for _, f := range d.Findings {
		lines = append(lines, Line{fmt.Sprintf("[%s] %s - %s.%s.%s", strings.ToUpper(f.Severity), f.Title, f.Database, f.Schema, f.Object), "subheading"})
		lines = append(lines, Line{"Evidência: " + f.Evidence, "body"}, Line{"Análise técnica: " + f.Summary, "body"}, Line{"Recomendação técnica: " + f.Recommendation, "body"}, Line{fmt.Sprintf("Confiança: %.0f%% | Regra: %s", f.Confidence*100, f.RuleVersion), "body"})
	}
	lines = append(lines, Line{Text: "Análise final e próximos passos", Style: "heading"})
	if len(d.Findings) == 0 {
		lines = append(lines, Line{Text: "Não há achados neste recorte. Confira a cobertura antes de concluir que o ambiente não possui riscos.", Style: "body"})
	} else {
		observation := "Foram observados"
		if len(d.Findings) == 1 {
			observation = "Foi observado"
		}
		lines = append(lines, Line{Text: fmt.Sprintf("%s %d %s neste recorte, incluindo %d %s e %d %s. Priorize a validação dos itens de maior severidade com a equipe responsável pelo banco.", observation, len(d.Findings), plural(len(d.Findings), "achado", "achados"), critical, plural(critical, "crítico", "críticos"), high, plural(high, "alto", "altos")), Style: "body"})
		for index, f := range d.Findings {
			if index >= 5 {
				break
			}
			lines = append(lines, Line{Text: fmt.Sprintf("Prioridade %d (%s): %s em %s.%s.%s. Próximo passo: %s", index+1, severityLabel(f.Severity), f.Title, f.Database, f.Schema, f.Object, f.Recommendation), Style: "body"})
		}
	}
	if d.Truncated || d.Coverage != "complete" {
		lines = append(lines, Line{Text: "A cobertura é parcial ou o conteúdo foi limitado. Execute uma coleta completa antes de decidir alterações com base na ausência de achados.", Style: "body"})
	}
	lines = append(lines, Line{Text: "Valide cada recomendação com métricas, plano de execução e teste em ambiente controlado. O auditor não aplica mudanças no banco auditado.", Style: "body"})
	lines = append(lines, Line{"Metodologia e glossário", "heading"}, Line{"Somente dados e observações da execução informada foram usados.", "body"}, Line{"Achado: sinal de diagnóstico que exige validação humana.", "body"}, Line{"Baseline: execução aprovada para comparação temporal.", "body"}, Line{"Cobertura parcial impede conclusões sobre ausência de objetos e achados.", "body"})
	return lines
}

func reportTypeLabel(kind string) string {
	switch kind {
	case "executive":
		return "executivo"
	case "technical":
		return "técnico"
	case "table":
		return "de tabela"
	default:
		return kind
	}
}

func plural(count int, singular, many string) string {
	if count == 1 {
		return singular
	}
	return many
}

func coverageLabel(value string) string {
	switch value {
	case "complete":
		return "completa"
	case "partial":
		return "parcial"
	default:
		return value
	}
}

func severityRank(severity string) int {
	switch strings.ToLower(severity) {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	case "low":
		return 3
	default:
		return 4
	}
}

func severityLabel(severity string) string {
	switch strings.ToLower(severity) {
	case "critical":
		return "crítica"
	case "high":
		return "alta"
	case "medium":
		return "média"
	case "low":
		return "baixa"
	default:
		return severity
	}
}

func scope(d Document) string {
	parts := []string{}
	for _, p := range []string{d.DatabaseFilter, d.SchemaFilter, d.TableFilter} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) == 0 {
		parts = append(parts, "ambiente inteiro")
	}
	if d.SeverityFilter != "" {
		parts = append(parts, "severidade: "+d.SeverityFilter)
	}
	return strings.Join(parts, " / ")
}

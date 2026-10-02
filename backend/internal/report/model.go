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
		return a.Severity+a.Category+a.Database+a.Schema+a.Object+a.Title < b.Severity+b.Category+b.Database+b.Schema+b.Object+b.Title
	})
	lines := []Line{{"DB Auditor - Relatorio " + strings.ToUpper(d.Type), "title"}, {"Ambiente: " + d.Environment, "body"}, {"Execucao: " + d.RunID, "body"}, {"Inicio: " + d.RunStarted.UTC().Format(time.RFC3339) + " | Estado: " + d.RunStatus, "body"}, {"Versao do servico: " + d.ServiceVersion + " | Regras: " + d.RuleVersion, "body"}, {"Solicitante: " + d.RequestedBy, "body"}, {"Escopo: " + scope(d), "body"}, {"", "body"}, {"Resumo executivo", "heading"}}
	critical, high := 0, 0
	for _, f := range d.Findings {
		if f.Severity == "critical" {
			critical++
		}
		if f.Severity == "high" {
			high++
		}
	}
	lines = append(lines, Line{fmt.Sprintf("%d bancos | %d tabelas | %d findings (%d criticos, %d altos)", len(d.Databases), d.TotalTables, d.TotalFindings, critical, high), "body"})
	lines = append(lines, Line{"Cobertura: " + d.Coverage, "body"})
	if d.Score != nil {
		lines = append(lines, Line{fmt.Sprintf("Score: %d/100 | confianca %.0f%%", *d.Score, d.ScoreConfidence*100), "body"})
	} else {
		lines = append(lines, Line{fmt.Sprintf("Score indisponivel | confianca %.0f%%", d.ScoreConfidence*100), "body"})
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
			lines = append(lines, Line{fmt.Sprintf("%s.%s.%s | %d bytes | %d linhas estimadas | PK %t", table.Database, table.Schema, table.Name, table.SizeBytes, table.Rows, table.HasPrimaryKey), "body"})
		}
	}
	lines = append(lines, Line{"Riscos e prioridades", "heading"})
	if len(d.Findings) == 0 {
		lines = append(lines, Line{"Nenhum finding observado neste escopo da execucao.", "body"})
	}
	for _, f := range d.Findings {
		lines = append(lines, Line{fmt.Sprintf("[%s] %s - %s.%s.%s", strings.ToUpper(f.Severity), f.Title, f.Database, f.Schema, f.Object), "subheading"})
		lines = append(lines, Line{"Evidencia: " + f.Evidence, "body"}, Line{"Analise: " + f.Summary, "body"}, Line{"Recomendacao: " + f.Recommendation, "body"}, Line{fmt.Sprintf("Confianca: %.0f%% | Regra: %s", f.Confidence*100, f.RuleVersion), "body"})
	}
	lines = append(lines, Line{"Metodologia e glossario", "heading"}, Line{"Somente snapshots e observacoes da execucao informada foram usados.", "body"}, Line{"Finding: sinal diagnostico, nao uma ordem de alteracao automatica.", "body"}, Line{"Baseline: execucao aprovada para comparacao temporal.", "body"}, Line{"Cobertura parcial impede conclusoes sobre ausencia de objetos e findings.", "body"})
	return lines
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

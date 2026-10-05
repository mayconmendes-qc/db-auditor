package report

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRenderPDFDeterministicAndPaginated(t *testing.T) {
	d := Document{Type: "technical", Environment: "Produção", RunID: "12345678-1234-4123-8123-123456789012", RunStarted: time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC), RunStatus: "success", ServiceVersion: "1.0", RuleVersion: "1.0", RequestedBy: "report-token", Coverage: "complete", Databases: []Database{{Name: "db", Schemas: 1, Tables: 200, SizeBytes: 1000}}, Baseline: &Baseline{RunID: "baseline", Status: "complete", AddedTables: 1, RemovedTables: 0, ChangedTables: 2}}
	for i := 0; i < 200; i++ {
		d.Tables = append(d.Tables, Table{Database: "db", Schema: "public", Name: strings.Repeat("orders", 5), SizeBytes: int64(i), Rows: 100, HasPrimaryKey: true})
	}
	d.Findings = []Finding{{Severity: "high", Category: "security", Database: "db", Schema: "public", Object: "orders", Title: "Permissão excessiva", Summary: "Revise privilégios", Recommendation: "Validar em ambiente controlado", Confidence: 0.9, Evidence: `{"role":"public"}`, RuleVersion: "1.0"}}
	first, err := RenderPDF(d)
	if err != nil {
		t.Fatal(err)
	}
	if err = ValidatePDF(first); err != nil {
		t.Fatal(err)
	}
	second, err := RenderPDF(d)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("same document did not produce identical PDF bytes")
	}
	if PageCount(first) < 2 {
		t.Fatalf("expected multiple pages; got %d", PageCount(first))
	}
	if path := os.Getenv("AUDITOR_REPORT_SAMPLE_PDF"); path != "" {
		if err = os.WriteFile(path, first, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestWrapTextBounded(t *testing.T) {
	for _, part := range wrapText(strings.Repeat("x", 220), 90) {
		if len(part) > 90 {
			t.Fatalf("unbounded part: %d", len(part))
		}
	}
}

func TestLargeInventoryPDFIsBounded(t *testing.T) {
	d := Document{Type: "technical", Environment: "large", RunID: "fixed", RunStarted: time.Unix(0, 0), RunStatus: "success", Coverage: "complete"}
	for i := 0; i < 5000; i++ {
		d.Tables = append(d.Tables, Table{Database: "db", Schema: "public", Name: strings.Repeat("large_table_", 3) + string(rune('a'+i%26)), SizeBytes: int64(i)})
	}
	pdf, err := RenderPDF(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(pdf) > maxPDFBytes || PageCount(pdf) > 200 {
		t.Fatalf("large report exceeded bounds: %d bytes, %d pages", len(pdf), PageCount(pdf))
	}
}

func TestReportContentGolden(t *testing.T) {
	d := Document{Type: "executive", Environment: "env", RunID: "run", RunStarted: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC), RunStatus: "partial_success", ServiceVersion: "1.0", RuleVersion: "rule-v2", RequestedBy: "report-token", Coverage: "partial", CoverageNotes: []string{"coverage incomplete"}, TotalTables: 4, TotalFindings: 1, Findings: []Finding{{Severity: "critical", Title: "risk", Recommendation: "validate", Evidence: "proof", Confidence: 0.8}}, Baseline: &Baseline{RunID: "base", Status: "partial"}, Growth: []GrowthPoint{{RunID: "run", SizeBytes: 100, Tables: 4}}}
	lines := BuildLines(d)
	joined := ""
	for _, line := range lines {
		joined += line.Text + "\n"
	}
	for _, want := range []string{"DB Auditor - Relatório executivo", "4 tabelas | 1 achado", "Cobertura: parcial", "Índice indisponível", "Baseline base", "Evolucao de armazenamento", "Evidência: proof", "Recomendação técnica: validate", "Metodologia e glossário"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("report missing %q", want)
		}
	}
	if !strings.Contains(joined, "Análise final e próximos passos") || !strings.Contains(joined, "Prioridade 1 (crítica): risk") {
		t.Fatal("report is missing its final analysis")
	}
	if !strings.Contains(string(mustRenderPDF(t, d)), " re f ") {
		t.Fatal("report is missing vector chart bars")
	}
}

func mustRenderPDF(t *testing.T, d Document) []byte {
	t.Helper()
	pdf, err := RenderPDF(d)
	if err != nil {
		t.Fatal(err)
	}
	return pdf
}

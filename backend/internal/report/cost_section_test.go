package report

import (
	"strings"
	"testing"
	"time"
)

func TestCostSectionStrictRedaction(t *testing.T) {
	amt := 1.5
	d := Document{
		Type: "technical", Environment: "erp", RunID: "run-1",
		RunStarted: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		CostSummary: &CostSummary{
			Priced: true, Currency: "BRL",
			StorageMonth: 10, UnusedIndexMonth: 1, BloatMonth: 2, QueryMonth: 0.5,
			Items: []CostItem{{Type: "unused_index", ObjectKey: "erp.public.secret_idx", Label: "estimado/mês", Amount: &amt}},
		},
	}
	d.RedactSensitive("strict")
	if d.CostSummary == nil || len(d.CostSummary.Items) != 0 {
		t.Fatalf("strict must drop object names: %+v", d.CostSummary)
	}
	lines := BuildLines(d)
	var text strings.Builder
	for _, line := range lines {
		text.WriteString(line.Text)
		text.WriteByte('\n')
	}
	body := text.String()
	if !strings.Contains(body, "Custo estimado") {
		t.Fatal("missing cost section")
	}
	if strings.Contains(body, "secret_idx") || strings.Contains(body, "erp.public") {
		t.Fatalf("object name leaked under strict redaction:\n%s", body)
	}
}

func TestCostSectionWithoutPrice(t *testing.T) {
	d := Document{
		Type: "executive", Environment: "erp", RunID: "run-1",
		RunStarted:  time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		CostSummary: &CostSummary{Priced: false, Reason: "não estimado — preço não configurado"},
	}
	lines := BuildLines(d)
	var text strings.Builder
	for _, line := range lines {
		text.WriteString(line.Text)
		text.WriteByte('\n')
	}
	if !strings.Contains(text.String(), "não estimado — preço não configurado") {
		t.Fatalf("missing no-price message:\n%s", text.String())
	}
}

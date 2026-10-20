package report

import (
	"strings"
	"testing"
	"time"
)

func TestRetentionSimulationSectionTechnical(t *testing.T) {
	amt := 12.5
	d := Document{
		Type: "technical", Environment: "erp", RunID: "run-1",
		RunStarted: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		RetentionSimulation: &RetentionSimulationSummary{
			Applicable: true,
			Items: []RetentionSimulationItem{{
				Hypertable: "public.metrics", Label: "ret-90", GBSaved: 310,
				Currency: "BRL", MoneySaved: &amt, Confidence: "média",
			}},
		},
	}
	body := linesText(BuildLines(d))
	if !strings.Contains(body, "Retenção e compressão (simulação)") {
		t.Fatal("missing section heading")
	}
	if !strings.Contains(body, "Hipótese:") || !strings.Contains(body, "310.00 GB") {
		t.Fatalf("missing hypothesis/gain:\n%s", body)
	}
}

func TestRetentionSimulationNotApplicable(t *testing.T) {
	d := Document{
		Type: "technical", Environment: "pg", RunID: "run-1",
		RunStarted:          time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		RetentionSimulation: &RetentionSimulationSummary{Applicable: false, Reason: "não aplicável"},
	}
	body := linesText(BuildLines(d))
	if !strings.Contains(body, "não aplicável") {
		t.Fatalf("want não aplicável:\n%s", body)
	}
}

func linesText(lines []Line) string {
	var b strings.Builder
	for _, line := range lines {
		b.WriteString(line.Text)
		b.WriteByte('\n')
	}
	return b.String()
}

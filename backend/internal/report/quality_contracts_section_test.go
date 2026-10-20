package report

import "testing"

func TestEmitQualityContractsTechnical(t *testing.T) {
	sample := 10000
	d := Document{
		Type: "technical",
		QualityContracts: &QualityContractsSummary{
			SamplingNote: "Margem de erro amostral desconhecida.",
			Items: []QualityContractLine{{
				CheckKey: "nulls:id", Result: "violated", Severity: "medium",
				SampleSize: &sample, Observed: "300 (3.00%)", Reason: "3.00% nulos > máximo 1.00%",
			}},
		},
	}
	var lines []string
	out := func(ls ...Line) error {
		for _, l := range ls {
			lines = append(lines, l.Text)
		}
		return nil
	}
	if err := emitQualityContracts(d, out); err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, l := range lines {
		joined += l + "\n"
	}
	if !containsAll(joined, "Contratos de qualidade", "nulls:id", "3.00%", "10000") {
		t.Fatalf("unexpected section:\n%s", joined)
	}
	if containsAll(joined, "2026-", "T00:") {
		t.Fatalf("must not include freshness timestamps: %s", joined)
	}
}

func TestEmitQualityContractsSkippedForNonTechnical(t *testing.T) {
	d := Document{Type: "executive", QualityContracts: &QualityContractsSummary{Items: []QualityContractLine{{CheckKey: "x"}}}}
	called := false
	_ = emitQualityContracts(d, func(...Line) error { called = true; return nil })
	if called {
		t.Fatal("executive report must omit quality contracts section")
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !containsStr(s, p) {
			return false
		}
	}
	return true
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

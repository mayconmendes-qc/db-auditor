package report

import "testing"

func TestEmitRestoreVerifiedEmpty(t *testing.T) {
	var lines []string
	out := func(ls ...Line) error {
		for _, l := range ls {
			lines = append(lines, l.Text)
		}
		return nil
	}
	d := Document{Type: "technical", RestoreVerified: &RestoreVerifiedSummary{HasDrill: false, MaxAgeDays: 30, Overdue: true}}
	if err := emitRestoreVerified(d, out); err != nil {
		t.Fatal(err)
	}
	if len(lines) < 2 {
		t.Fatalf("%v", lines)
	}
	if lines[0] != "Restauração verificada" {
		t.Fatalf("%q", lines[0])
	}
}

func TestEmitRestoreVerifiedWithDrill(t *testing.T) {
	rto := 120
	var lines []string
	out := func(ls ...Line) error {
		for _, l := range ls {
			lines = append(lines, l.Text)
		}
		return nil
	}
	d := Document{Type: "executive", RestoreVerified: &RestoreVerifiedSummary{
		HasDrill: true, Succeeded: true, PerformedAt: "2026-10-01T00:00:00Z",
		RTOSeconds: &rto, RTOTargetSeconds: 600, ChecksOK: 3, ChecksTotal: 4, DivergenceCount: 0,
	}}
	if err := emitRestoreVerified(d, out); err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, l := range lines {
		joined += l + "\n"
	}
	if !contains(joined, "sucesso") || !contains(joined, "120 s") {
		t.Fatalf("%s", joined)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || (len(s) > 0 && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()))
}

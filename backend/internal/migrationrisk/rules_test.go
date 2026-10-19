package migrationrisk

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMR01NoContextMediumLowConfidence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "01_idx.sql")
	if err := os.WriteFile(path, []byte("CREATE INDEX idx_orders_email ON public.orders (email);\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rep, err := AnalyzePaths([]string{path}, Context{Mode: "none", PGVersion: 16}, DefaultThresholds())
	if err != nil {
		t.Fatal(err)
	}
	if rep.Summary.Highest != SeverityMedium {
		t.Fatalf("highest=%s want medium", rep.Summary.Highest)
	}
	var found bool
	for _, f := range rep.Findings {
		if f.Code == "MR-01" {
			found = true
			if f.Severity != SeverityMedium || f.Confidence != ConfidenceLow {
				t.Fatalf("MR-01 sev=%s conf=%s", f.Severity, f.Confidence)
			}
			if !strings.Contains(f.Impact, "não estimado") {
				t.Fatalf("impact=%q", f.Impact)
			}
		}
	}
	if !found {
		t.Fatal("MR-01 missing")
	}
	if code := GateExit(rep, SeverityHigh, nil); code != 0 {
		t.Fatalf("exit=%d want 0", code)
	}
}

func TestMR01CriticalWithSnapshotContext(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "01_idx.sql")
	if err := os.WriteFile(path, []byte("CREATE INDEX idx_orders_email ON public.orders (email);\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	size := int64(12) << 30
	ctx := Context{
		Mode: "snapshot", PGVersion: 16,
		Tables: map[string]TableContext{
			"public.orders": {Qualified: "public.orders", SizeBytes: size, RowEstimate: 60_000_000},
		},
	}
	rep, err := AnalyzePaths([]string{path}, ctx, DefaultThresholds())
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, f := range rep.Findings {
		if f.Code == "MR-01" {
			found = true
			if f.Severity != SeverityCritical {
				t.Fatalf("severity=%s", f.Severity)
			}
			if f.Lock != "SHARE" {
				t.Fatalf("lock=%s", f.Lock)
			}
			if !strings.Contains(strings.ToUpper(f.Alternative), "CONCURRENTLY") {
				t.Fatalf("alt=%s", f.Alternative)
			}
			if f.SizeBytes == nil || *f.SizeBytes != size {
				t.Fatalf("size=%v", f.SizeBytes)
			}
		}
	}
	if !found {
		t.Fatal("MR-01 missing")
	}
	if code := GateExit(rep, SeverityHigh, nil); code != 1 {
		t.Fatalf("exit=%d want 1", code)
	}
}

func TestMigrationOutputHasNoLiterals(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "seed.sql")
	sql := "UPDATE public.users SET email = 'person@example.com' WHERE id = 1;\nCREATE INDEX idx ON public.orders(id);\n"
	if err := os.WriteFile(path, []byte(sql), 0o600); err != nil {
		t.Fatal(err)
	}
	rep, err := AnalyzePaths([]string{path}, Context{Mode: "none", PGVersion: 16}, DefaultThresholds())
	if err != nil {
		t.Fatal(err)
	}
	md := RenderMarkdown(rep)
	raw, _ := os.ReadFile(path)
	_ = raw
	blob := md
	for _, f := range rep.Findings {
		blob += f.Object + f.Alternative + f.Justification + f.Operation
	}
	if strings.Contains(blob, "person@example.com") {
		t.Fatal("literal leaked into output")
	}
}

func TestInlineSuppressionWithJustification(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ok.sql")
	sql := "-- db-auditor:ignore MR-01 motivo: janela de manutenção aprovada\nCREATE INDEX idx ON public.orders(id);\n"
	if err := os.WriteFile(path, []byte(sql), 0o600); err != nil {
		t.Fatal(err)
	}
	rep, err := AnalyzePaths([]string{path}, Context{Mode: "none", PGVersion: 16}, DefaultThresholds())
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, f := range rep.Findings {
		if f.Code == "MR-01" {
			found = true
			if !f.Suppressed || f.Justification == "" {
				t.Fatalf("expected suppression: %+v", f)
			}
		}
	}
	if !found {
		t.Fatal("MR-01 missing")
	}
	if code := GateExit(rep, SeverityHigh, nil); code != 0 {
		t.Fatalf("suppressed should not gate: %d", code)
	}
}

func TestSuppressionWithoutJustificationFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.sql")
	sql := "-- db-auditor:ignore MR-01\nCREATE INDEX idx ON public.orders(id);\n"
	if err := os.WriteFile(path, []byte(sql), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := AnalyzePaths([]string{path}, Context{Mode: "none", PGVersion: 16}, DefaultThresholds())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLockTablePerMajorMR04(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nn.sql")
	sql := "ALTER TABLE public.orders ALTER COLUMN email SET NOT NULL;\n"
	if err := os.WriteFile(path, []byte(sql), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, major := range []int{14, 15, 16, 17, 18} {
		rep, err := AnalyzePaths([]string{path}, Context{Mode: "none", PGVersion: major}, DefaultThresholds())
		if err != nil {
			t.Fatal(err)
		}
		var alt string
		for _, f := range rep.Findings {
			if f.Code == "MR-04" {
				alt = f.Alternative
			}
		}
		if alt == "" {
			t.Fatalf("MR-04 missing for pg %d", major)
		}
		if major >= 18 {
			if !strings.Contains(alt, "NOT VALID") || !strings.Contains(alt, "18") {
				t.Fatalf("pg18 alt=%q", alt)
			}
		} else {
			if !strings.Contains(alt, "CHECK") || !strings.Contains(alt, "VALIDATE") {
				t.Fatalf("pg%d alt=%q", major, alt)
			}
		}
	}
}

func TestDeterministicOutput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "d.sql")
	if err := os.WriteFile(path, []byte("CREATE INDEX a ON public.t(x);\nALTER TABLE public.t ALTER COLUMN y SET NOT NULL;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	a, err := AnalyzePaths([]string{path}, Context{Mode: "none", PGVersion: 16}, DefaultThresholds())
	if err != nil {
		t.Fatal(err)
	}
	b, err := AnalyzePaths([]string{path}, Context{Mode: "none", PGVersion: 16}, DefaultThresholds())
	if err != nil {
		t.Fatal(err)
	}
	if RenderMarkdown(a) != RenderMarkdown(b) {
		t.Fatal("non-deterministic markdown")
	}
}

func TestPartialSnapshotExit3(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "p.sql")
	_ = os.WriteFile(path, []byte("CREATE INDEX a ON public.t(x);\n"), 0o600)
	rep, err := AnalyzePaths([]string{path}, Context{Mode: "snapshot", Partial: true, PGVersion: 16}, DefaultThresholds())
	if err != nil {
		t.Fatal(err)
	}
	if code := GateExit(rep, SeverityHigh, nil); code != 3 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(RenderMarkdown(rep), "parcial") {
		t.Fatal("markdown should mention partial context")
	}
}

func FuzzMigrationLexer(f *testing.F) {
	f.Add("CREATE INDEX i ON t(a);")
	f.Add("ALTER TABLE t SET NOT NULL; -- db-auditor:ignore MR-04 motivo: x")
	f.Add("E'str''ing' $tag$dollar$tag$ /* c */")
	f.Fuzz(func(t *testing.T, src string) {
		defer func() {
			if recover() != nil {
				t.Fatal("lexer panicked")
			}
		}()
		_ = SplitStatements(src)
		_, _ = ParseSuppressions(src)
	})
}

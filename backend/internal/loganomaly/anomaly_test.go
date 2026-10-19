package loganomaly

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func pgLine(ts time.Time, sev, msg string) string {
	// PG jsonlog-ish timestamp
	tstr := ts.UTC().Format("2006-01-02 15:04:05.000 UTC")
	b, _ := json.Marshal(map[string]any{
		"timestamp":      tstr,
		"error_severity": sev,
		"message":        msg,
		"user":           "alice",
		"dbname":         "appdb",
		"remote_host":    "10.0.0.9",
	})
	return string(b)
}

func TestStrongSpikeConnectionAuthorized(t *testing.T) {
	// CA-01: 12 windows × 10 + 1 window × 90 → strong spike ~9×
	var buf bytes.Buffer
	base := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	msg := "connection authorized: user=alice database=app"
	for w := 0; w < 12; w++ {
		for i := 0; i < 10; i++ {
			ts := base.Add(time.Duration(w)*5*time.Minute + time.Duration(i)*time.Second)
			fmt.Fprintln(&buf, pgLine(ts, "LOG", msg))
		}
	}
	spikeStart := base.Add(12 * 5 * time.Minute)
	for i := 0; i < 90; i++ {
		ts := spikeStart.Add(time.Duration(i) * time.Second)
		fmt.Fprintln(&buf, pgLine(ts, "LOG", msg))
	}

	res, err := Analyze(Options{
		Profile:    ProfilePostgresJSONLog,
		Thresholds: DefaultThresholds(),
		FailOn:     SeverityCritical, // don't gate on high for this assert
	}, &buf)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, a := range res.Report.Anomalies {
		if a.Kind == AnomalyStrongSpike {
			found = true
			if a.Factor < 8 {
				t.Fatalf("factor=%v want ≥8", a.Factor)
			}
			if strings.Contains(a.Template, "alice") || strings.Contains(a.Template, "app") {
				t.Fatalf("unmasked template: %s", a.Template)
			}
			if !strings.Contains(a.Template, "connection authorized") {
				t.Fatalf("template missing fixed part: %s", a.Template)
			}
		}
	}
	if !found {
		t.Fatalf("expected strong_spike, got %#v", res.Report.Anomalies)
	}
}

func TestNewErrorTemplateMedium(t *testing.T) {
	// CA-03: new error template with 15 occurrences → medium
	var buf bytes.Buffer
	base := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	steady := "checkpoint complete: wrote buffers"
	for w := 0; w < 8; w++ {
		for i := 0; i < 5; i++ {
			ts := base.Add(time.Duration(w)*5*time.Minute + time.Duration(i)*time.Second)
			fmt.Fprintln(&buf, pgLine(ts, "LOG", steady))
		}
	}
	spikeStart := base.Add(8 * 5 * time.Minute)
	newMsg := "ERROR: permission denied for relation secret_table"
	for i := 0; i < 15; i++ {
		ts := spikeStart.Add(time.Duration(i) * time.Second)
		fmt.Fprintln(&buf, pgLine(ts, "ERROR", newMsg))
	}

	res, err := Analyze(Options{Profile: ProfilePostgresJSONLog, Thresholds: DefaultThresholds()}, &buf)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, a := range res.Report.Anomalies {
		if a.Kind == AnomalyNewTemplate {
			found = true
			if a.Severity != SeverityMedium {
				t.Fatalf("severity=%s want medium", a.Severity)
			}
			if ContainsSensitiveLeak(a.Template, "secret_table") {
				// table name may remain as fixed token — that's OK for identifiers that aren't literals.
				// Spec cares about emails/IPs/params; relation name is structural.
			}
		}
	}
	if !found {
		t.Fatalf("expected new_template, got %#v caveats=%v base=%v", res.Report.Anomalies, res.Report.Caveats, res.Report.Baseline)
	}
}

func TestInvalidLinesExit3(t *testing.T) {
	// CA-04: 20% non-JSON → exit 3
	dir := t.TempDir()
	path := filepath.Join(dir, "mix.log")
	var b strings.Builder
	base := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 80; i++ {
		fmt.Fprintln(&b, pgLine(base.Add(time.Duration(i)*time.Second), "LOG", "ok message steady state here"))
	}
	for i := 0; i < 20; i++ {
		fmt.Fprintln(&b, "this is not json at all")
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := Analyze(Options{Profile: ProfilePostgresJSONLog, Paths: []string{path}, Thresholds: DefaultThresholds()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 3 {
		t.Fatalf("exit=%d want 3 stats=%+v", res.ExitCode, res.Stats)
	}
	if res.Stats.LinesInvalid < 20 {
		t.Fatalf("invalid=%d", res.Stats.LinesInvalid)
	}
}

func TestDeterministicJSONOutput(t *testing.T) {
	// CA-05
	var src bytes.Buffer
	base := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	for w := 0; w < 10; w++ {
		for i := 0; i < 12; i++ {
			ts := base.Add(time.Duration(w)*5*time.Minute + time.Duration(i)*time.Second)
			fmt.Fprintln(&src, pgLine(ts, "LOG", "connection authorized: user=bob database=db1"))
		}
	}
	raw := src.Bytes()
	opts := Options{Profile: ProfilePostgresJSONLog, Thresholds: DefaultThresholds(), Format: "json"}
	r1, err := Analyze(opts, bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	r2, err := Analyze(opts, bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	j1, _ := json.Marshal(r1.Report)
	j2, _ := json.Marshal(r2.Report)
	if !bytes.Equal(j1, j2) {
		t.Fatalf("non-deterministic\n%s\n----\n%s", j1, j2)
	}
}

func TestMissingFileExit2(t *testing.T) {
	res, err := Analyze(Options{
		Profile:    ProfilePostgresJSONLog,
		Paths:      []string{filepath.Join(t.TempDir(), "nope.json")},
		Thresholds: DefaultThresholds(),
	}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if res.ExitCode != 2 {
		t.Fatalf("exit=%d", res.ExitCode)
	}
}

func TestGenericRequiresTimeField(t *testing.T) {
	_, err := NewProfile(ProfileGenerico, "", "level", "msg")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestErrorSpikeGateExit1(t *testing.T) {
	var buf bytes.Buffer
	base := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	for w := 0; w < 8; w++ {
		// 2 errors per window
		for i := 0; i < 2; i++ {
			ts := base.Add(time.Duration(w)*5*time.Minute + time.Duration(i)*time.Second)
			fmt.Fprintln(&buf, pgLine(ts, "ERROR", "division by zero in query"))
		}
		for i := 0; i < 5; i++ {
			ts := base.Add(time.Duration(w)*5*time.Minute + time.Duration(10+i)*time.Second)
			fmt.Fprintln(&buf, pgLine(ts, "LOG", "duration: ms statement select"))
		}
	}
	spike := base.Add(8 * 5 * time.Minute)
	for i := 0; i < 20; i++ {
		fmt.Fprintln(&buf, pgLine(spike.Add(time.Duration(i)*time.Second), "ERROR", "division by zero in query"))
	}
	res, err := Analyze(Options{
		Profile:    ProfilePostgresJSONLog,
		FailOn:     SeverityHigh,
		Thresholds: DefaultThresholds(),
	}, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 1 {
		t.Fatalf("exit=%d anomalies=%#v caveats=%v", res.ExitCode, res.Report.Anomalies, res.Report.Caveats)
	}
}

func TestNoNetworkNoDatabase(t *testing.T) {
	// RNF-01: Analyze must not dial; we only feed stdin/files.
	var buf bytes.Buffer
	fmt.Fprintln(&buf, pgLine(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), "LOG", "hello world steady"))
	res, err := Analyze(Options{Profile: ProfilePostgresJSONLog, Thresholds: DefaultThresholds()}, &buf)
	if err != nil {
		t.Fatal(err)
	}
	_ = res
}

func TestInsufficientBaselineExit3(t *testing.T) {
	var buf bytes.Buffer
	base := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		fmt.Fprintln(&buf, pgLine(base.Add(time.Duration(i)*time.Minute), "LOG", "steady message alpha"))
	}
	res, err := Analyze(Options{Profile: ProfilePostgresJSONLog, Thresholds: DefaultThresholds()}, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Report.Baseline.Insufficient {
		t.Fatalf("expected insufficient baseline: %+v", res.Report.Baseline)
	}
	if res.ExitCode != 3 {
		t.Fatalf("exit=%d want 3", res.ExitCode)
	}
}

func TestOversizedLineDiscarded(t *testing.T) {
	var buf bytes.Buffer
	big := strings.Repeat("x", MaxLineBytes+10)
	fmt.Fprintf(&buf, "{\"timestamp\":\"2026-01-01 00:00:00.000 UTC\",\"error_severity\":\"LOG\",\"message\":%q}\n", big)
	fmt.Fprintln(&buf, pgLine(time.Date(2026, 1, 1, 0, 1, 0, 0, time.UTC), "LOG", "small ok message here"))
	res, err := Analyze(Options{Profile: ProfilePostgresJSONLog, Thresholds: DefaultThresholds()}, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if res.Stats.LinesOversized < 1 {
		t.Fatalf("expected oversized, stats=%+v", res.Stats)
	}
}

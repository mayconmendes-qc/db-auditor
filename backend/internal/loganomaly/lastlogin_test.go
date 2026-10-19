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

func TestLastLoginOnlyRoleAndTime(t *testing.T) {
	// CA-07
	var buf bytes.Buffer
	t1 := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 10, 1, 11, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

	line := func(ts time.Time, user, msg string) string {
		b, _ := json.Marshal(map[string]any{
			"timestamp":        ts.Format("2006-01-02 15:04:05.000 UTC"),
			"error_severity":   "LOG",
			"message":          msg,
			"user":             user,
			"dbname":           "secretdb",
			"remote_host":      "203.0.113.50",
			"application_name": "psql",
		})
		return string(b)
	}
	fmt.Fprintln(&buf, line(t1, "alice", "connection authorized: user=alice database=secretdb"))
	fmt.Fprintln(&buf, line(t2, "bob", "connection authorized: user=bob database=secretdb"))
	fmt.Fprintln(&buf, line(t3, "alice", "connection authorized: user=alice database=secretdb"))
	fmt.Fprintln(&buf, line(t3, "alice", "statement: SELECT 1")) // not a login

	out := filepath.Join(t.TempDir(), "ultimo-login.json")
	res, err := Analyze(Options{
		Profile:      ProfilePostgresJSONLog,
		LastLoginOut: out,
		Thresholds:   DefaultThresholds(),
	}, &buf)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateLastLoginOnlyRoleAndTime(raw); err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{"203.0.113.50", "secretdb", "psql", "remote_host", "dbname", "application"} {
		if strings.Contains(string(raw), leak) {
			t.Fatalf("leak %q in %s", leak, raw)
		}
	}
	var f LastLoginFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if f.SchemaVersion != SchemaVersion {
		t.Fatalf("schema %s", f.SchemaVersion)
	}
	byRole := map[string]string{}
	for _, e := range f.Entries {
		byRole[e.Role] = e.LastLoginAt
	}
	if byRole["alice"] != t3.Format(time.RFC3339) {
		t.Fatalf("alice last=%s", byRole["alice"])
	}
	if byRole["bob"] != t2.Format(time.RFC3339) {
		t.Fatalf("bob last=%s", byRole["bob"])
	}
	if res.Report.LastLogin == nil || len(res.Report.LastLogin.Entries) != 2 {
		t.Fatalf("report last_login=%v", res.Report.LastLogin)
	}
}

func TestLastLoginRejectedForGenerico(t *testing.T) {
	var buf bytes.Buffer
	fmt.Fprintln(&buf, `{"ts":"2026-01-01T00:00:00Z","level":"info","msg":"hello world"}`)
	res, err := Analyze(Options{
		Profile:      ProfileGenerico,
		TimeField:    "ts",
		LevelField:   "level",
		MessageField: "msg",
		LastLoginOut: filepath.Join(t.TempDir(), "x.json"),
		Thresholds:   DefaultThresholds(),
	}, &buf)
	if err == nil {
		t.Fatal("expected error")
	}
	if res.ExitCode != 2 {
		t.Fatalf("exit=%d", res.ExitCode)
	}
}

func TestReportNeverEmitsRawNeedles(t *testing.T) {
	var buf bytes.Buffer
	base := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	needles := []string{"person@example.com", "192.168.0.42", "supersecret"}
	msg := "ERROR: login person@example.com from 192.168.0.42 user=supersecret failed"
	for w := 0; w < 8; w++ {
		for i := 0; i < 12; i++ {
			ts := base.Add(time.Duration(w)*5*time.Minute + time.Duration(i)*time.Second)
			fmt.Fprintln(&buf, pgLine(ts, "ERROR", msg))
		}
	}
	res, err := Analyze(Options{Profile: ProfilePostgresJSONLog, Thresholds: DefaultThresholds()}, &buf)
	if err != nil {
		t.Fatal(err)
	}
	md := RenderMarkdown(res.Report)
	js, _ := json.Marshal(res.Report)
	for _, n := range needles {
		if strings.Contains(md, n) || strings.Contains(string(js), n) {
			t.Fatalf("needle %q leaked", n)
		}
	}
}

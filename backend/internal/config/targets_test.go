package config

import (
	"os"
	"strings"
	"testing"
)

func TestTargetDSNKey(t *testing.T) {
	t.Parallel()
	got := TargetDSNKey("00000000-0000-0000-0000-000000000001")
	want := "AUDITOR_TARGET_DSN_00000000_0000_0000_0000_000000000001"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestBuildTargetDSN(t *testing.T) {
	t.Parallel()
	dsn, err := BuildTargetDSN("db.example.com", "5432", "tsdb", "ro_user", "p@ss:word/x", "require")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dsn, "db.example.com:5432") {
		t.Fatalf("host/port missing: %s", dsn)
	}
	if !strings.Contains(dsn, "/tsdb") {
		t.Fatalf("database missing: %s", dsn)
	}
	if !strings.Contains(dsn, "sslmode=require") {
		t.Fatalf("sslmode missing: %s", dsn)
	}
	// password must be URL-encoded
	if strings.Contains(dsn, "p@ss:word/x") {
		t.Fatalf("password not encoded: %s", dsn)
	}
}

func TestLoadDiscreteTargetDSNs(t *testing.T) {
	id := "00000000-0000-0000-0000-000000000001"
	keys := []string{
		"AUDITOR_TARGET_1_ENVIRONMENT_ID",
		"AUDITOR_TARGET_1_HOST",
		"AUDITOR_TARGET_1_PORT",
		"AUDITOR_TARGET_1_DATABASE",
		"AUDITOR_TARGET_1_USER",
		"AUDITOR_TARGET_1_PASSWORD",
		"AUDITOR_TARGET_1_SSLMODE",
	}
	for _, k := range keys {
		_ = os.Unsetenv(k)
	}
	_ = os.Setenv("AUDITOR_TARGET_1_ENVIRONMENT_ID", id)
	_ = os.Setenv("AUDITOR_TARGET_1_HOST", "host.example")
	_ = os.Setenv("AUDITOR_TARGET_1_PORT", "6543")
	_ = os.Setenv("AUDITOR_TARGET_1_DATABASE", "mydb")
	_ = os.Setenv("AUDITOR_TARGET_1_USER", "ro")
	_ = os.Setenv("AUDITOR_TARGET_1_PASSWORD", "secret")
	_ = os.Setenv("AUDITOR_TARGET_1_SSLMODE", "require")
	t.Cleanup(func() {
		for _, k := range keys {
			_ = os.Unsetenv(k)
		}
	})

	m := LoadTargetDSNs()
	dsn, ok := m[id]
	if !ok || dsn == "" {
		t.Fatalf("expected discrete DSN for %s, got map=%v", id, m)
	}
	if !strings.Contains(dsn, "host.example:6543") || !strings.Contains(dsn, "/mydb") {
		t.Fatalf("unexpected dsn: %s", dsn)
	}
	if got := DSNForEnvironment(nil, id); got != dsn {
		t.Fatalf("DSNForEnvironment nil map: got %q want %q", got, dsn)
	}
}

func TestDSNForEnvironmentLegacy(t *testing.T) {
	id := "00000000-0000-0000-0000-000000000099"
	key := TargetDSNKey(id)
	want := "postgresql://user:pass@legacy-host:5432/db?sslmode=disable"
	_ = os.Setenv(key, want)
	t.Cleanup(func() { _ = os.Unsetenv(key) })

	m := LoadTargetDSNs()
	if got := DSNForEnvironment(m, id); got != want {
		t.Fatalf("from map: got %q want %q", got, want)
	}
	if got := DSNForEnvironment(nil, id); got != want {
		t.Fatalf("from env: got %q want %q", got, want)
	}
}

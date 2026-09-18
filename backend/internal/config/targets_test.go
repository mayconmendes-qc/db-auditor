package config

import (
	"os"
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

func TestDSNForEnvironment(t *testing.T) {
	key := TargetDSNKey("00000000-0000-0000-0000-000000000001")
	_ = os.Setenv(key, "postgresql://user:pass@host:5432/db")
	t.Cleanup(func() { _ = os.Unsetenv(key) })

	dsn := DSNForEnvironment(nil, "00000000-0000-0000-0000-000000000001")
	if dsn == "" {
		t.Fatal("expected dsn from env")
	}
}

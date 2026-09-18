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
	id := "00000000-0000-0000-0000-000000000001"
	key := TargetDSNKey(id)
	want := "postgresql://user:pass@host:5432/db"
	_ = os.Setenv(key, want)
	t.Cleanup(func() { _ = os.Unsetenv(key) })

	// Via map
	m := LoadTargetDSNs()
	if got := DSNForEnvironment(m, id); got != want {
		t.Fatalf("from map: got %q want %q", got, want)
	}
	// Via env fallback when map is nil
	if got := DSNForEnvironment(nil, id); got != want {
		t.Fatalf("from env: got %q want %q", got, want)
	}
}

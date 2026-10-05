package buildinfo

import "testing"

func TestString(t *testing.T) {
	version, commit := Version, Commit
	defer func() {
		Version, Commit = version, commit
	}()

	Version = "1.4.0"
	Commit = "unknown"
	if got := String(); got != Version {
		t.Fatalf("String() with unknown commit = %q, want %q", got, Version)
	}

	Commit = "abcdef0123456789abcdef0123456789abcdef01"
	want := Version + "+" + Commit[:12]
	if got := String(); got != want {
		t.Fatalf("String() with 40-char commit = %q, want %q", got, want)
	}
}

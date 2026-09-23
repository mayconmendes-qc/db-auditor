package config

import (
	"errors"
	"strings"
	"testing"
)

func TestSanitizeDSN_URI(t *testing.T) {
	t.Parallel()
	in := "postgresql://auditor:s3cret@db.example:5432/app?sslmode=require"
	got := SanitizeDSN(in)
	if strings.Contains(got, "s3cret") {
		t.Fatalf("password leaked: %s", got)
	}
	if !strings.Contains(got, "auditor") {
		t.Fatalf("username should remain: %s", got)
	}
	if !strings.Contains(got, "***") {
		t.Fatalf("expected redaction marker: %s", got)
	}
	if !strings.Contains(got, "db.example") {
		t.Fatalf("host should remain: %s", got)
	}
}

func TestSanitizeDSN_ErrorText(t *testing.T) {
	t.Parallel()
	in := `connect failed: failed to connect to postgresql://u:p@h/db: password authentication failed`
	got := SanitizeDSN(in)
	if strings.Contains(got, ":p@") || strings.Contains(got, "u:p@") {
		t.Fatalf("password leaked: %s", got)
	}
}

func TestSanitizeError(t *testing.T) {
	t.Parallel()
	err := errors.New("dial postgresql://x:y@z/db: timeout")
	got := SanitizeError(err)
	if strings.Contains(got, ":y@") {
		t.Fatalf("leaked: %s", got)
	}
}

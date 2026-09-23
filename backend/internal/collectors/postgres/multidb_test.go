package postgres

import (
	"strings"
	"testing"
)

func TestRewriteDatabase(t *testing.T) {
	t.Parallel()
	got, err := rewriteDatabase("postgres://user:pass@host:5432/postgres?sslmode=require", "app_db")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "/app_db") {
		t.Fatalf("path not rewritten: %s", got)
	}
	if !strings.Contains(got, "sslmode=require") {
		t.Fatalf("query lost: %s", got)
	}
}

func TestFormatPartialErrors(t *testing.T) {
	t.Parallel()
	if FormatPartialErrors(nil, 3) != "" {
		t.Fatal("expected empty")
	}
	partial := []PartialError{
		{Database: "a", Op: "connect", Message: "denied"},
		{Database: "b", Op: "collect", Message: "timeout"},
		{Database: "c", Op: "collect", Message: "x"},
	}
	s := FormatPartialErrors(partial, 2)
	if !strings.Contains(s, "a[connect]") || !strings.Contains(s, "(+1)") {
		t.Fatalf("unexpected: %s", s)
	}
}

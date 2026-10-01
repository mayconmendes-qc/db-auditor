package config

import (
	"strings"
	"testing"
)

func TestSanitizeConnectionString(t *testing.T) {
	raw := "postgresql://auditor:s3cret@postgres:5432/timescale_auditor?sslmode=disable&password=also"
	got := SanitizeConnectionString(raw)
	if strings.Contains(got, "s3cret") || strings.Contains(got, "also") {
		t.Fatalf("secret leaked in sanitized string: %s", got)
	}
}

func TestScopeAllowsDatabase(t *testing.T) {
	s := Scope{
		DatabaseDenylist:  []string{"template0", "postgres"},
		DatabaseAllowlist: []string{"app_db"},
	}
	if s.AllowsDatabase("postgres") {
		t.Fatal("denylist should block postgres")
	}
	if !s.AllowsDatabase("app_db") {
		t.Fatal("allowlist should permit app_db")
	}
	if s.AllowsDatabase("other") {
		t.Fatal("non-allowlisted name should be blocked when allowlist is set")
	}
}

func TestScopeAllowsSchema(t *testing.T) {
	s := Scope{SchemaDenylist: []string{"pg_catalog"}}
	if s.AllowsSchema("pg_catalog") {
		t.Fatal("denylist should block pg_catalog")
	}
	if !s.AllowsSchema("public") {
		t.Fatal("public should be allowed by default")
	}
}

func TestColumnStatsCollectionPolicy(t *testing.T) {
	t.Setenv("AUDITOR_DATABASE_URL", "postgres://auditor:local@localhost:5432/auditor")
	t.Setenv("AUDITOR_COLUMN_STATS_ENABLED", "false")
	t.Setenv("AUDITOR_COLUMN_STATS_LIMIT", "42")
	t.Setenv("AUDITOR_COLUMN_STATS_SCHEMA_ALLOWLIST", "public,analytics")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Collection.ColumnStatsEnabled || cfg.Collection.ColumnStatsLimit != 42 || len(cfg.Collection.ColumnStatsSchemas) != 2 || cfg.Collection.ColumnStatsSchemas[1] != "analytics" {
		t.Fatalf("unexpected column stats policy: %#v", cfg.Collection)
	}
}

package config

import "testing"

func TestSanitizeConnectionString(t *testing.T) {
	raw := "postgresql://auditor:s3cret@postgres:5432/timescale_auditor?sslmode=disable&password=also"
	got := SanitizeConnectionString(raw)
	if contains(got, "s3cret") || contains(got, "also") {
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

func contains(s, sub string) bool {
	return len(sub) > 0 && (s == sub || len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})())
}

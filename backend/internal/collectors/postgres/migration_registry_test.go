package postgres

import (
	"strings"
	"testing"
)

func TestValidateMigrationIdentifier(t *testing.T) {
	if err := ValidateMigrationIdentifier("schema_migrations"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMigrationIdentifier(""); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{`x"; drop table t--`, "a b", "1abc", "has-dash", strings.Repeat("a", 64)} {
		if err := ValidateMigrationIdentifier(bad); err == nil {
			t.Fatalf("expected reject for %q", bad)
		}
	}
}

func TestDefaultTablePerTool(t *testing.T) {
	cases := map[MigrationTool]string{
		ToolGolangMigrate: "schema_migrations",
		ToolGoose:         "goose_db_version",
		ToolFlyway:        "flyway_schema_history",
		ToolAlembic:       "alembic_version",
		ToolRails:         "schema_migrations",
		ToolPrisma:        "_prisma_migrations",
	}
	for tool, want := range cases {
		_, table := DefaultTable(tool)
		if table != want {
			t.Fatalf("%s: got %s want %s", tool, table, want)
		}
	}
}

func TestToolsWithoutTimestampDocumented(t *testing.T) {
	// golang-migrate, Alembic, Rails correlate by version change only (ADR-0001 D5).
	for _, tool := range []MigrationTool{ToolGolangMigrate, ToolAlembic, ToolRails} {
		_, table := DefaultTable(tool)
		if table == "" {
			t.Fatalf("missing default for %s", tool)
		}
	}
}

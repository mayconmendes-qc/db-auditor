package postgres

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var identifierRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// MigrationTool is one of the six ADR-0001 D5 tools.
type MigrationTool string

const (
	ToolGolangMigrate MigrationTool = "golang-migrate"
	ToolGoose         MigrationTool = "goose"
	ToolFlyway        MigrationTool = "flyway"
	ToolAlembic       MigrationTool = "alembic"
	ToolRails         MigrationTool = "rails"
	ToolPrisma        MigrationTool = "prisma"
)

// MigrationRegistryConfig is the operator-configured table to read.
type MigrationRegistryConfig struct {
	Tool       MigrationTool
	SchemaName string // optional; default public / empty
	TableName  string // optional; tool default when empty
}

// MigrationRegistryReading is the sanitized snapshot (version + optional stamp only).
type MigrationRegistryReading struct {
	Tool          MigrationTool
	LatestVersion string
	AppliedAt     *time.Time
	AccessState   string // ok | missing | denied | error
}

// DefaultTable returns the conventional table name for a tool.
func DefaultTable(tool MigrationTool) (schema, table string) {
	switch tool {
	case ToolGolangMigrate, ToolRails:
		return "public", "schema_migrations"
	case ToolGoose:
		return "public", "goose_db_version"
	case ToolFlyway:
		return "public", "flyway_schema_history"
	case ToolAlembic:
		return "public", "alembic_version"
	case ToolPrisma:
		return "public", "_prisma_migrations"
	default:
		return "public", ""
	}
}

// ValidateMigrationIdentifier rejects malicious schema/table names (R2).
func ValidateMigrationIdentifier(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	if !identifierRE.MatchString(name) {
		return fmt.Errorf("identificador inválido")
	}
	if len(name) > 63 {
		return fmt.Errorf("identificador inválido")
	}
	return nil
}

// CollectMigrationRegistry reads only the latest version and optional applied_at.
// Failures never abort the audit run: AccessState captures missing/denied/error.
func CollectMigrationRegistry(ctx context.Context, conn *pgx.Conn, cfg MigrationRegistryConfig) MigrationRegistryReading {
	out := MigrationRegistryReading{Tool: cfg.Tool, AccessState: "error"}
	schema, table := cfg.SchemaName, cfg.TableName
	if schema == "" || table == "" {
		ds, dt := DefaultTable(cfg.Tool)
		if schema == "" {
			schema = ds
		}
		if table == "" {
			table = dt
		}
	}
	if err := ValidateMigrationIdentifier(schema); err != nil {
		out.AccessState = "error"
		return out
	}
	if err := ValidateMigrationIdentifier(table); err != nil {
		out.AccessState = "error"
		return out
	}
	qual := pgx.Identifier{schema, table}.Sanitize()

	var exists bool
	if err := conn.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1 FROM pg_catalog.pg_class c
  JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
  WHERE n.nspname=$1 AND c.relname=$2 AND c.relkind IN ('r','p')
)`, schema, table).Scan(&exists); err != nil {
		out.AccessState = "error"
		return out
	}
	if !exists {
		out.AccessState = "missing"
		return out
	}

	var q string
	switch cfg.Tool {
	case ToolGolangMigrate, ToolRails:
		q = fmt.Sprintf(`SELECT version::text, NULL::timestamptz FROM %s ORDER BY version DESC LIMIT 1`, qual)
	case ToolGoose:
		q = fmt.Sprintf(`SELECT version_id::text, tstamp FROM %s ORDER BY version_id DESC LIMIT 1`, qual)
	case ToolFlyway:
		q = fmt.Sprintf(`SELECT version::text, installed_on FROM %s WHERE success ORDER BY installed_rank DESC LIMIT 1`, qual)
	case ToolAlembic:
		q = fmt.Sprintf(`SELECT version_num::text, NULL::timestamptz FROM %s LIMIT 1`, qual)
	case ToolPrisma:
		q = fmt.Sprintf(`SELECT migration_name::text, finished_at FROM %s WHERE finished_at IS NOT NULL ORDER BY finished_at DESC NULLS LAST LIMIT 1`, qual)
	default:
		out.AccessState = "error"
		return out
	}

	var version string
	var applied *time.Time
	if err := conn.QueryRow(ctx, q).Scan(&version, &applied); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "permission") || strings.Contains(err.Error(), "42501") {
			out.AccessState = "denied"
			return out
		}
		// goose without is_applied column
		if cfg.Tool == ToolGoose {
			q2 := fmt.Sprintf(`SELECT version_id::text, tstamp FROM %s ORDER BY id DESC LIMIT 1`, qual)
			if err2 := conn.QueryRow(ctx, q2).Scan(&version, &applied); err2 == nil {
				out.LatestVersion = version
				out.AppliedAt = applied
				out.AccessState = "ok"
				return out
			}
		}
		out.AccessState = "error"
		return out
	}
	out.LatestVersion = version
	out.AppliedAt = applied
	out.AccessState = "ok"
	return out
}

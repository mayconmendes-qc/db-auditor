package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/osmendes/db-auditor/internal/collectors/postgres"
)

// EnvironmentMigrationRegistry is operator config for RF-05(b).
type EnvironmentMigrationRegistry struct {
	ID            string    `json:"id"`
	EnvironmentID string    `json:"environment_id"`
	DatabaseName  string    `json:"database_name"`
	Tool          string    `json:"tool"`
	SchemaName    string    `json:"schema_name"`
	TableName     string    `json:"table_name"`
	Enabled       bool      `json:"enabled"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// MigrationRegistryInput is PUT body item.
type MigrationRegistryInput struct {
	DatabaseName string `json:"database_name"`
	Tool         string `json:"tool"`
	SchemaName   string `json:"schema_name"`
	TableName    string `json:"table_name"`
	Enabled      bool   `json:"enabled"`
}

var allowedMigrationTools = map[string]struct{}{
	"golang-migrate": {}, "goose": {}, "flyway": {}, "alembic": {}, "rails": {}, "prisma": {},
}

// ValidateMigrationRegistryInput checks tool + identifiers (400 on malice).
func ValidateMigrationRegistryInput(in MigrationRegistryInput) error {
	tool := strings.TrimSpace(strings.ToLower(in.Tool))
	if _, ok := allowedMigrationTools[tool]; !ok {
		return fmt.Errorf("ferramenta de migration inválida")
	}
	if err := postgres.ValidateMigrationIdentifier(in.SchemaName); err != nil {
		return fmt.Errorf("schema inválido")
	}
	if err := postgres.ValidateMigrationIdentifier(in.TableName); err != nil {
		return fmt.Errorf("tabela inválida")
	}
	if strings.ContainsAny(in.SchemaName+in.TableName, `";'`) {
		return fmt.Errorf("identificador inválido")
	}
	return nil
}

// ListMigrationRegistry returns configs for an environment.
func (s *Store) ListMigrationRegistry(ctx context.Context, env string) ([]EnvironmentMigrationRegistry, error) {
	rows, err := s.pool.Query(ctx, `
SELECT id::text, environment_id::text, database_name, tool, schema_name, table_name, enabled, updated_at
FROM environment_migration_registry
WHERE environment_id=$1::uuid
ORDER BY database_name, tool`, env)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EnvironmentMigrationRegistry{}
	for rows.Next() {
		var r EnvironmentMigrationRegistry
		if err := rows.Scan(&r.ID, &r.EnvironmentID, &r.DatabaseName, &r.Tool, &r.SchemaName, &r.TableName, &r.Enabled, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ReplaceMigrationRegistry replaces all configs for an environment.
func (s *Store) ReplaceMigrationRegistry(ctx context.Context, env, actor string, inputs []MigrationRegistryInput) ([]EnvironmentMigrationRegistry, error) {
	for _, in := range inputs {
		if err := ValidateMigrationRegistryInput(in); err != nil {
			return nil, err
		}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `DELETE FROM environment_migration_registry WHERE environment_id=$1::uuid`, env); err != nil {
		return nil, err
	}
	for _, in := range inputs {
		tool := strings.TrimSpace(strings.ToLower(in.Tool))
		_, err = tx.Exec(ctx, `
INSERT INTO environment_migration_registry(environment_id, database_name, tool, schema_name, table_name, enabled, updated_at)
VALUES($1::uuid,$2,$3,$4,$5,$6,now())`,
			env, strings.TrimSpace(in.DatabaseName), tool, strings.TrimSpace(in.SchemaName), strings.TrimSpace(in.TableName), in.Enabled)
		if err != nil {
			return nil, err
		}
	}
	if _, err = tx.Exec(ctx, `
INSERT INTO auditor_operation_log(username, action, environment_id, result)
VALUES($1,'migration_registry.replace',$2::uuid,'success')`, actor, env); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.ListMigrationRegistry(ctx, env)
}

// SaveMigrationRegistrySnapshot persists one reading for a run.
func (s *Store) SaveMigrationRegistrySnapshot(ctx context.Context, env, run, database string, reading postgres.MigrationRegistryReading) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO migration_registry_snapshot(audit_run_id, environment_id, database_name, tool, latest_version, applied_at, access_state)
VALUES($1::uuid,$2::uuid,$3,$4,$5,$6,$7)
ON CONFLICT (audit_run_id, database_name, tool) DO UPDATE SET
  latest_version=EXCLUDED.latest_version, applied_at=EXCLUDED.applied_at,
  access_state=EXCLUDED.access_state, collected_at=now()`,
		run, env, database, string(reading.Tool), reading.LatestVersion, reading.AppliedAt, reading.AccessState)
	return err
}

// ListEnabledMigrationRegistryForEnv returns enabled configs (collector use).
func (s *Store) ListEnabledMigrationRegistryForEnv(ctx context.Context, env string) ([]EnvironmentMigrationRegistry, error) {
	all, err := s.ListMigrationRegistry(ctx, env)
	if err != nil {
		return nil, err
	}
	out := all[:0]
	for _, r := range all {
		if r.Enabled {
			out = append(out, r)
		}
	}
	return out, nil
}

-- Optional application migration-registry correlation (spec 04 / ADR-0001 D5).
-- Idempotent.

CREATE TABLE IF NOT EXISTS environment_migration_registry (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  environment_id uuid NOT NULL REFERENCES audit_environment(id) ON DELETE CASCADE,
  database_name text NOT NULL DEFAULT '',
  tool text NOT NULL CHECK (tool IN (
    'golang-migrate', 'goose', 'flyway', 'alembic', 'rails', 'prisma'
  )),
  schema_name text NOT NULL DEFAULT '',
  table_name text NOT NULL DEFAULT '',
  enabled boolean NOT NULL DEFAULT true,
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (environment_id, database_name, tool)
);

CREATE TABLE IF NOT EXISTS migration_registry_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id) ON DELETE CASCADE,
  database_name text NOT NULL DEFAULT '',
  tool text NOT NULL,
  latest_version text NOT NULL DEFAULT '',
  applied_at timestamptz,
  access_state text NOT NULL DEFAULT 'ok'
    CHECK (access_state IN ('ok', 'missing', 'denied', 'error')),
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name, tool)
);

CREATE INDEX IF NOT EXISTS migration_registry_snapshot_env_run_idx
  ON migration_registry_snapshot (environment_id, audit_run_id);

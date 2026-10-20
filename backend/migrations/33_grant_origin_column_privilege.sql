-- Grant origin + column privileges (spec 07 RF-02, RF-03).
-- Snapshots antigos ficam "não coletado" até a próxima coleta completa.
-- grant_snapshot permanece inalterado (compatibilidade).

CREATE TABLE IF NOT EXISTS grant_origin_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  account_name text NOT NULL,
  object_type text NOT NULL
    CHECK (object_type IN ('table', 'view', 'sequence', 'function', 'schema')),
  object_name text NOT NULL,
  privilege text NOT NULL,
  origin_kind text NOT NULL
    CHECK (origin_kind IN ('direct', 'role', 'public', 'predefined')),
  via_chain text[] NOT NULL DEFAULT '{}',
  -- PG 16+: whether the membership path used INHERIT (USAGE) and/or SET.
  inherit_path boolean,
  set_path boolean,
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name, account_name, object_type, object_name, privilege)
);

CREATE INDEX IF NOT EXISTS grant_origin_snapshot_env_run_idx
  ON grant_origin_snapshot (environment_id, audit_run_id, account_name, object_name);

CREATE INDEX IF NOT EXISTS grant_origin_snapshot_origin_idx
  ON grant_origin_snapshot (environment_id, audit_run_id, origin_kind);

CREATE TABLE IF NOT EXISTS column_grant_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  schema_name text NOT NULL,
  table_name text NOT NULL,
  column_name text NOT NULL,
  account_name text NOT NULL,
  privilege text NOT NULL,
  origin_kind text NOT NULL
    CHECK (origin_kind IN ('direct', 'role', 'public', 'predefined')),
  via_chain text[] NOT NULL DEFAULT '{}',
  inherit_path boolean,
  set_path boolean,
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name, schema_name, table_name, column_name, account_name, privilege)
);

CREATE INDEX IF NOT EXISTS column_grant_snapshot_env_run_idx
  ON column_grant_snapshot (environment_id, audit_run_id, schema_name, table_name);

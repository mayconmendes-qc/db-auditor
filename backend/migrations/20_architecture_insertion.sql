-- Read-only target probes are stored only in the DB Auditor control database.
CREATE TABLE IF NOT EXISTS architecture_insertion_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  database_name text NOT NULL,
  schema_name text NOT NULL,
  table_name text NOT NULL,
  column_name text NOT NULL,
  decision_version integer NOT NULL,
  append_only boolean NOT NULL,
  status text NOT NULL CHECK (status IN ('observed','empty','skipped_budget','permission_denied','failed','invalid_mapping')),
  reason text NOT NULL DEFAULT '',
  latest_at timestamptz,
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id,database_name,schema_name,table_name)
);
CREATE INDEX IF NOT EXISTS architecture_insertion_snapshot_scope_idx
  ON architecture_insertion_snapshot(environment_id,audit_run_id,database_name,schema_name);

-- Sprint 14 — structural inventory: relation classification + constraint FK metadata.
-- Safe additive migration (nullable columns / new tables).

ALTER TABLE table_snapshot
  ADD COLUMN IF NOT EXISTS relation_class text,
  ADD COLUMN IF NOT EXISTS is_partition boolean NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS parent_schema_name text,
  ADD COLUMN IF NOT EXISTS parent_table_name text,
  ADD COLUMN IF NOT EXISTS partition_bound text,
  ADD COLUMN IF NOT EXISTS tablespace_name text,
  ADD COLUMN IF NOT EXISTS relpersistence text,
  ADD COLUMN IF NOT EXISTS relrowsecurity boolean,
  ADD COLUMN IF NOT EXISTS relforcerowsecurity boolean,
  ADD COLUMN IF NOT EXISTS table_comment text;

ALTER TABLE constraint_snapshot
  ADD COLUMN IF NOT EXISTS constrained_columns text[],
  ADD COLUMN IF NOT EXISTS referenced_schema_name text,
  ADD COLUMN IF NOT EXISTS referenced_table_name text,
  ADD COLUMN IF NOT EXISTS referenced_columns text[],
  ADD COLUMN IF NOT EXISTS fk_update_action text,
  ADD COLUMN IF NOT EXISTS fk_delete_action text,
  ADD COLUMN IF NOT EXISTS fk_match_type text;

CREATE TABLE IF NOT EXISTS sequence_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL,
  database_name text NOT NULL,
  schema_name text NOT NULL,
  sequence_name text NOT NULL,
  data_type text,
  start_value numeric,
  increment_by numeric,
  max_value numeric,
  min_value numeric,
  cycle boolean,
  owned_by_table text,
  owned_by_column text,
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name, schema_name, sequence_name)
);

CREATE TABLE IF NOT EXISTS trigger_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL,
  database_name text NOT NULL,
  schema_name text NOT NULL,
  table_name text NOT NULL,
  trigger_name text NOT NULL,
  enabled text,
  timing text,
  event_manipulation text,
  action_statement text,
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name, schema_name, table_name, trigger_name)
);

CREATE TABLE IF NOT EXISTS policy_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL,
  database_name text NOT NULL,
  schema_name text NOT NULL,
  table_name text NOT NULL,
  policy_name text NOT NULL,
  permissive text,
  roles text[],
  cmd text,
  qual text,
  with_check text,
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name, schema_name, table_name, policy_name)
);

CREATE INDEX IF NOT EXISTS idx_table_snapshot_relation_class
  ON table_snapshot (environment_id, relation_class);

CREATE INDEX IF NOT EXISTS idx_constraint_snapshot_type
  ON constraint_snapshot (environment_id, constraint_type);

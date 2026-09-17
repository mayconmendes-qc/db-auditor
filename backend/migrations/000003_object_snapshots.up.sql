CREATE TABLE table_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  schema_name text NOT NULL,
  table_name text NOT NULL,
  owner_name text,
  relkind text NOT NULL DEFAULT 'r',
  data_size_bytes bigint NOT NULL DEFAULT 0,
  index_size_bytes bigint NOT NULL DEFAULT 0,
  total_size_bytes bigint NOT NULL DEFAULT 0,
  row_estimate bigint NOT NULL DEFAULT 0,
  n_live_tup bigint NOT NULL DEFAULT 0,
  n_dead_tup bigint NOT NULL DEFAULT 0,
  n_tup_ins bigint NOT NULL DEFAULT 0,
  n_tup_upd bigint NOT NULL DEFAULT 0,
  n_tup_del bigint NOT NULL DEFAULT 0,
  seq_scan bigint NOT NULL DEFAULT 0,
  idx_scan bigint NOT NULL DEFAULT 0,
  last_vacuum timestamptz,
  last_autovacuum timestamptz,
  last_analyze timestamptz,
  last_autoanalyze timestamptz,
  column_count integer NOT NULL DEFAULT 0,
  has_primary_key boolean NOT NULL DEFAULT false,
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name, schema_name, table_name)
);

CREATE TABLE column_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  schema_name text NOT NULL,
  table_name text NOT NULL,
  column_name text NOT NULL,
  ordinal_position integer NOT NULL,
  data_type text NOT NULL,
  is_nullable boolean NOT NULL DEFAULT true,
  column_default text,
  is_generated boolean NOT NULL DEFAULT false,
  identity_generation text,
  collation_name text,
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name, schema_name, table_name, column_name)
);

CREATE TABLE index_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  schema_name text NOT NULL,
  table_name text NOT NULL,
  index_name text NOT NULL,
  index_definition text NOT NULL,
  access_method text,
  is_unique boolean NOT NULL DEFAULT false,
  is_primary boolean NOT NULL DEFAULT false,
  size_bytes bigint NOT NULL DEFAULT 0,
  idx_scan bigint NOT NULL DEFAULT 0,
  idx_tup_read bigint NOT NULL DEFAULT 0,
  idx_tup_fetch bigint NOT NULL DEFAULT 0,
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name, schema_name, index_name)
);

CREATE INDEX table_snapshot_env_run_idx ON table_snapshot (environment_id, audit_run_id);
CREATE INDEX table_snapshot_db_schema_idx ON table_snapshot (database_name, schema_name);
CREATE INDEX column_snapshot_env_run_idx ON column_snapshot (environment_id, audit_run_id);
CREATE INDEX column_snapshot_table_idx ON column_snapshot (database_name, schema_name, table_name);
CREATE INDEX index_snapshot_env_run_idx ON index_snapshot (environment_id, audit_run_id);
CREATE INDEX index_snapshot_table_idx ON index_snapshot (database_name, schema_name, table_name);

CREATE TABLE database_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  owner_name text,
  encoding text,
  collate_name text,
  ctype_name text,
  allow_connections boolean NOT NULL DEFAULT true,
  is_template boolean NOT NULL DEFAULT false,
  size_bytes bigint NOT NULL DEFAULT 0,
  connection_count integer NOT NULL DEFAULT 0,
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name)
);

CREATE TABLE schema_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  schema_name text NOT NULL,
  owner_name text,
  table_count integer NOT NULL DEFAULT 0,
  view_count integer NOT NULL DEFAULT 0,
  materialized_view_count integer NOT NULL DEFAULT 0,
  sequence_count integer NOT NULL DEFAULT 0,
  function_count integer NOT NULL DEFAULT 0,
  size_bytes bigint NOT NULL DEFAULT 0,
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name, schema_name)
);

CREATE INDEX database_snapshot_env_run_idx ON database_snapshot (environment_id, audit_run_id);
CREATE INDEX schema_snapshot_env_run_idx ON schema_snapshot (environment_id, audit_run_id);
CREATE INDEX schema_snapshot_db_idx ON schema_snapshot (database_name, schema_name);

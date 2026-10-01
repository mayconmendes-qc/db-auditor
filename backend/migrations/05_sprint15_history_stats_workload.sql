-- Sprint 15: historical series, privacy-safe column statistics and workload facts.
-- Additive migration; audited databases remain strictly read-only.

CREATE INDEX IF NOT EXISTS table_snapshot_history_idx
  ON table_snapshot (environment_id, database_name, schema_name, table_name, collected_at DESC);

CREATE INDEX IF NOT EXISTS table_snapshot_run_history_idx
  ON table_snapshot (environment_id, audit_run_id, collected_at DESC);

CREATE TABLE column_stat_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  schema_name text NOT NULL,
  table_name text NOT NULL,
  column_name text NOT NULL,
  null_fraction double precision NOT NULL DEFAULT 0,
  distinct_estimate double precision NOT NULL DEFAULT 0,
  average_width integer NOT NULL DEFAULT 0,
  correlation double precision,
  source text NOT NULL DEFAULT 'pg_stats',
  quality text NOT NULL DEFAULT 'estimate',
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name, schema_name, table_name, column_name)
);

CREATE INDEX column_stat_snapshot_object_idx
  ON column_stat_snapshot (environment_id, database_name, schema_name, table_name, collected_at DESC);

CREATE TABLE workload_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  query_fingerprint text NOT NULL,
  query_id text NOT NULL DEFAULT '',
  calls bigint NOT NULL DEFAULT 0,
  total_exec_time_ms double precision NOT NULL DEFAULT 0,
  mean_exec_time_ms double precision NOT NULL DEFAULT 0,
  rows_total bigint NOT NULL DEFAULT 0,
  shared_blocks_read bigint NOT NULL DEFAULT 0,
  shared_blocks_hit bigint NOT NULL DEFAULT 0,
  referenced_objects text[] NOT NULL DEFAULT '{}',
  stats_reset timestamptz,
  source text NOT NULL DEFAULT 'pg_stat_statements',
  evidence_quality text NOT NULL DEFAULT 'aggregate',
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name, query_fingerprint)
);

CREATE INDEX workload_snapshot_object_idx
  ON workload_snapshot USING gin (referenced_objects);

CREATE INDEX workload_snapshot_history_idx
  ON workload_snapshot (environment_id, database_name, collected_at DESC);

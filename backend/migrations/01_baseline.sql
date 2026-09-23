-- DB Auditor — snapshot store baseline (MVP 0.12+)
-- Applied once by Postgres docker-entrypoint-initdb.d on empty volume.
-- After changing this file in development: make reset-volume && make up

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Core control plane
CREATE TABLE audit_environment (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL UNIQUE,
  type text NOT NULL CHECK (type IN ('tiger_cloud', 'self_hosted')),
  discovery_mode text NOT NULL CHECK (discovery_mode IN ('single_database', 'multi_database')),
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE audit_run (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  profile text NOT NULL CHECK (profile IN ('fast', 'daily', 'weekly', 'monthly', 'manual')),
  status text NOT NULL CHECK (status IN ('running', 'success', 'partial_success', 'failed', 'cancelled')),
  service_version text NOT NULL,
  collector_version text NOT NULL,
  started_at timestamptz NOT NULL DEFAULT now(),
  finished_at timestamptz,
  warnings jsonb NOT NULL DEFAULT '[]'::jsonb,
  errors jsonb NOT NULL DEFAULT '[]'::jsonb
);

CREATE TABLE collector_run (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id),
  collector_name text NOT NULL,
  collector_version text NOT NULL,
  status text NOT NULL CHECK (status IN ('running', 'success', 'skipped', 'failed')),
  query_name text,
  started_at timestamptz NOT NULL DEFAULT now(),
  finished_at timestamptz,
  rows_collected bigint NOT NULL DEFAULT 0,
  warning text,
  error text
);

CREATE INDEX audit_run_environment_started_at_idx ON audit_run (environment_id, started_at DESC);
CREATE INDEX collector_run_audit_run_id_idx ON collector_run (audit_run_id);

-- Topology snapshots
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

-- Object snapshots
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

CREATE TABLE constraint_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  schema_name text NOT NULL,
  table_name text NOT NULL,
  constraint_name text NOT NULL,
  constraint_type text NOT NULL,
  constraint_definition text NOT NULL,
  is_validated boolean NOT NULL DEFAULT true,
  is_deferrable boolean NOT NULL DEFAULT false,
  is_deferred boolean NOT NULL DEFAULT false,
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name, schema_name, constraint_name)
);

CREATE TABLE view_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  schema_name text NOT NULL,
  view_name text NOT NULL,
  owner_name text,
  relkind text NOT NULL,
  view_definition text NOT NULL,
  size_bytes bigint NOT NULL DEFAULT 0,
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name, schema_name, view_name)
);

CREATE TABLE function_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  schema_name text NOT NULL,
  function_name text NOT NULL,
  identity_arguments text NOT NULL DEFAULT '',
  owner_name text,
  language_name text,
  is_security_definer boolean NOT NULL DEFAULT false,
  volatility text,
  parallel_safety text,
  kind text,
  function_definition text,
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name, schema_name, function_name, identity_arguments)
);

CREATE TABLE extension_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  extension_name text NOT NULL,
  extension_version text,
  schema_name text,
  is_relocatable boolean NOT NULL DEFAULT false,
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name, extension_name)
);

CREATE INDEX constraint_snapshot_env_run_idx ON constraint_snapshot (environment_id, audit_run_id);
CREATE INDEX view_snapshot_env_run_idx ON view_snapshot (environment_id, audit_run_id);
CREATE INDEX function_snapshot_env_run_idx ON function_snapshot (environment_id, audit_run_id);
CREATE INDEX extension_snapshot_env_run_idx ON extension_snapshot (environment_id, audit_run_id);

-- Timescale core
CREATE TABLE timescale_version_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  extension_name text NOT NULL,
  extension_version text NOT NULL,
  schema_name text,
  major int NOT NULL DEFAULT 0,
  minor int NOT NULL DEFAULT 0,
  patch int NOT NULL DEFAULT 0,
  compatible boolean NOT NULL DEFAULT false,
  compatibility_note text,
  collection_status text NOT NULL DEFAULT 'OK',
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name)
);

CREATE TABLE hypertable_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  schema_name text NOT NULL,
  hypertable_name text NOT NULL,
  owner_name text,
  num_dimensions int NOT NULL DEFAULT 0,
  num_chunks int NOT NULL DEFAULT 0,
  compression_enabled boolean NOT NULL DEFAULT false,
  is_distributed boolean NOT NULL DEFAULT false,
  total_size_bytes bigint NOT NULL DEFAULT 0,
  data_size_bytes bigint NOT NULL DEFAULT 0,
  index_size_bytes bigint NOT NULL DEFAULT 0,
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name, schema_name, hypertable_name)
);

CREATE TABLE dimension_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  schema_name text NOT NULL,
  hypertable_name text NOT NULL,
  dimension_number int NOT NULL,
  column_name text NOT NULL,
  column_type text,
  dimension_type text,
  time_interval text,
  integer_interval text,
  integer_now_func text,
  num_slices int,
  partitioning_func text,
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name, schema_name, hypertable_name, dimension_number)
);

CREATE TABLE chunk_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  schema_name text NOT NULL,
  hypertable_name text NOT NULL,
  chunk_schema text NOT NULL,
  chunk_name text NOT NULL,
  range_start timestamptz,
  range_end timestamptz,
  range_start_integer bigint,
  range_end_integer bigint,
  is_compressed boolean NOT NULL DEFAULT false,
  chunk_tablespace text,
  total_size_bytes bigint NOT NULL DEFAULT 0,
  data_size_bytes bigint NOT NULL DEFAULT 0,
  index_size_bytes bigint NOT NULL DEFAULT 0,
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name, chunk_schema, chunk_name)
);

CREATE INDEX timescale_version_snapshot_env_run_idx ON timescale_version_snapshot (environment_id, audit_run_id);
CREATE INDEX hypertable_snapshot_env_run_idx ON hypertable_snapshot (environment_id, audit_run_id);
CREATE INDEX dimension_snapshot_env_run_idx ON dimension_snapshot (environment_id, audit_run_id);
CREATE INDEX chunk_snapshot_env_run_idx ON chunk_snapshot (environment_id, audit_run_id);
CREATE INDEX chunk_snapshot_ht_idx ON chunk_snapshot (audit_run_id, database_name, schema_name, hypertable_name);

-- Policies / jobs / CAGG
CREATE TABLE continuous_aggregate_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  schema_name text NOT NULL,
  view_name text NOT NULL,
  owner_name text,
  materialization_schema text,
  materialization_hypertable text,
  materialized_only boolean NOT NULL DEFAULT false,
  compression_enabled boolean NOT NULL DEFAULT false,
  finalized boolean,
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name, schema_name, view_name)
);

CREATE TABLE job_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  job_id bigint NOT NULL,
  application_name text,
  schedule_interval text,
  max_runtime text,
  max_retries int NOT NULL DEFAULT 0,
  retry_period text,
  proc_schema text,
  proc_name text,
  owner_name text,
  scheduled boolean NOT NULL DEFAULT false,
  fixed_schedule boolean NOT NULL DEFAULT false,
  config_json text,
  next_start timestamptz,
  initial_start timestamptz,
  hypertable_schema text,
  hypertable_name text,
  check_schema text,
  check_name text,
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name, job_id)
);

CREATE TABLE policy_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  job_id bigint NOT NULL,
  policy_type text NOT NULL,
  proc_schema text,
  proc_name text,
  hypertable_schema text,
  hypertable_name text,
  schedule_interval text,
  scheduled boolean NOT NULL DEFAULT false,
  config_json text,
  next_start timestamptz,
  owner_name text,
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name, job_id)
);

CREATE INDEX continuous_aggregate_snapshot_env_run_idx ON continuous_aggregate_snapshot (environment_id, audit_run_id);
CREATE INDEX job_snapshot_env_run_idx ON job_snapshot (environment_id, audit_run_id);
CREATE INDEX policy_snapshot_env_run_idx ON policy_snapshot (environment_id, audit_run_id);
CREATE INDEX policy_snapshot_type_idx ON policy_snapshot (audit_run_id, policy_type);

-- Mappings
CREATE TABLE object_mapping (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  source_environment_id uuid NOT NULL REFERENCES audit_environment(id),
  target_environment_id uuid NOT NULL REFERENCES audit_environment(id),
  source_database text NOT NULL,
  source_schema text NOT NULL DEFAULT '',
  source_object_type text NOT NULL,
  source_object_name text NOT NULL,
  target_database text NOT NULL,
  target_schema text NOT NULL DEFAULT '',
  target_object_type text NOT NULL,
  target_object_name text NOT NULL,
  relation_type text NOT NULL CHECK (relation_type IN ('database_to_schema', 'identical', 'renamed', 'unmatched')),
  confidence numeric(4,3) NOT NULL DEFAULT 0,
  status text NOT NULL CHECK (status IN ('suggested', 'validated', 'rejected', 'manual')) DEFAULT 'manual',
  source_fingerprint text,
  target_fingerprint text,
  fingerprint_algorithm text,
  notes text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT object_mapping_envs_distinct CHECK (source_environment_id <> target_environment_id)
);

CREATE INDEX object_mapping_source_env_idx ON object_mapping (source_environment_id);
CREATE INDEX object_mapping_target_env_idx ON object_mapping (target_environment_id);
CREATE INDEX object_mapping_status_idx ON object_mapping (status);
CREATE UNIQUE INDEX object_mapping_unique_pair_idx ON object_mapping (
  source_environment_id, target_environment_id,
  source_database, source_schema, source_object_type, source_object_name,
  target_database, target_schema, target_object_type, target_object_name
);

-- Findings
CREATE TABLE finding (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  audit_run_id uuid REFERENCES audit_run(id),
  finding_type text NOT NULL,
  severity text NOT NULL CHECK (severity IN ('info', 'low', 'medium', 'high', 'critical')),
  status text NOT NULL CHECK (status IN ('open', 'acknowledged', 'resolved', 'suppressed')) DEFAULT 'open',
  title text NOT NULL,
  summary text NOT NULL DEFAULT '',
  object_type text NOT NULL DEFAULT '',
  object_key text NOT NULL DEFAULT '',
  database_name text NOT NULL DEFAULT '',
  schema_name text NOT NULL DEFAULT '',
  object_name text NOT NULL DEFAULT '',
  evidence jsonb NOT NULL DEFAULT '{}'::jsonb,
  dedup_key text NOT NULL,
  first_seen_at timestamptz NOT NULL DEFAULT now(),
  last_seen_at timestamptz NOT NULL DEFAULT now(),
  resolved_at timestamptz,
  notes text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX finding_dedup_idx ON finding (environment_id, dedup_key);
CREATE INDEX finding_env_status_idx ON finding (environment_id, status);
CREATE INDEX finding_type_idx ON finding (finding_type);
CREATE INDEX finding_severity_idx ON finding (severity);
CREATE INDEX finding_last_seen_idx ON finding (last_seen_at DESC);

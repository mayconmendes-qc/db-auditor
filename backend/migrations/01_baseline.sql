-- DB Auditor — snapshot store baseline
-- Applied once by Postgres docker-entrypoint-initdb.d on an empty volume.
-- Sprints 13–20 were folded into this file. Do not add another incremental
-- chain here: a new install is this file plus 02_seed_demo.sql.
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

-- Folded from sprints 13–20. Columns below used to be later ALTER TABLE statements.

ALTER TABLE index_snapshot ADD COLUMN stats_reset timestamptz;
ALTER TABLE table_snapshot ADD COLUMN stats_reset timestamptz;
ALTER TABLE job_snapshot ADD COLUMN last_run_status text;
ALTER TABLE job_snapshot ADD COLUMN total_failures bigint NOT NULL DEFAULT 0;
ALTER TABLE continuous_aggregate_snapshot ADD COLUMN view_definition text NOT NULL DEFAULT '';

CREATE TABLE audit_run_coverage (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  collector_name text NOT NULL,
  database_name text NOT NULL DEFAULT '',
  status text NOT NULL CHECK (status IN ('attempted', 'success', 'skipped', 'failed')),
  rows_collected bigint NOT NULL DEFAULT 0,
  warning text,
  error text,
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, collector_name, database_name)
);

CREATE INDEX audit_run_coverage_run_idx
  ON audit_run_coverage (audit_run_id, collector_name, database_name);

CREATE TABLE analysis_run (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  status text NOT NULL CHECK (status IN ('running', 'success', 'failed')),
  analyzer_version text NOT NULL,
  findings_produced integer NOT NULL DEFAULT 0,
  findings_saved integer NOT NULL DEFAULT 0,
  error text,
  started_at timestamptz NOT NULL DEFAULT now(),
  finished_at timestamptz,
  rule_manifest_hash text NOT NULL DEFAULT '',
  UNIQUE (audit_run_id)
);

CREATE INDEX analysis_run_environment_started_idx
  ON analysis_run (environment_id, started_at DESC);

ALTER TABLE table_snapshot
  ADD COLUMN relation_class text,
  ADD COLUMN is_partition boolean NOT NULL DEFAULT false,
  ADD COLUMN parent_schema_name text,
  ADD COLUMN parent_table_name text,
  ADD COLUMN partition_bound text,
  ADD COLUMN tablespace_name text,
  ADD COLUMN relpersistence text,
  ADD COLUMN relrowsecurity boolean,
  ADD COLUMN relforcerowsecurity boolean,
  ADD COLUMN table_comment text,
  ADD COLUMN storage_parameters text[] NOT NULL DEFAULT ARRAY[]::text[];

ALTER TABLE constraint_snapshot
  ADD COLUMN constrained_columns text[],
  ADD COLUMN referenced_schema_name text,
  ADD COLUMN referenced_table_name text,
  ADD COLUMN referenced_columns text[],
  ADD COLUMN fk_update_action text,
  ADD COLUMN fk_delete_action text,
  ADD COLUMN fk_match_type text;

-- policy_snapshot already exists above and belongs to TimescaleDB jobs.
-- Row-level security policies use rls_policy_snapshot, created later.

CREATE TABLE sequence_snapshot (
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

CREATE TABLE trigger_snapshot (
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

CREATE INDEX idx_table_snapshot_relation_class
  ON table_snapshot (environment_id, relation_class);

CREATE INDEX idx_constraint_snapshot_type
  ON constraint_snapshot (environment_id, constraint_type);

CREATE INDEX table_snapshot_history_idx
  ON table_snapshot (environment_id, database_name, schema_name, table_name, collected_at DESC);

CREATE INDEX table_snapshot_run_history_idx
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
  extension_version text NOT NULL DEFAULT '',
  query_kind text NOT NULL DEFAULT 'other',
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name, query_fingerprint)
);

CREATE INDEX workload_snapshot_object_idx
  ON workload_snapshot USING gin (referenced_objects);

CREATE INDEX workload_snapshot_history_idx
  ON workload_snapshot (environment_id, database_name, collected_at DESC);

ALTER TABLE index_snapshot
  ADD COLUMN is_valid boolean NOT NULL DEFAULT true,
  ADD COLUMN is_ready boolean NOT NULL DEFAULT true,
  ADD COLUMN key_columns text[] NOT NULL DEFAULT '{}',
  ADD COLUMN predicate text NOT NULL DEFAULT '';

CREATE TABLE rule_catalog (
  rule_id text NOT NULL,
  rule_version text NOT NULL,
  category text NOT NULL,
  confidence numeric(4,3) NOT NULL CHECK (confidence BETWEEN 0 AND 1),
  impact text NOT NULL,
  risk text NOT NULL,
  recommendation text NOT NULL,
  validation text NOT NULL,
  references_json jsonb NOT NULL DEFAULT '[]'::jsonb,
  default_parameters jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (rule_id, rule_version)
);

CREATE TABLE rule_policy (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  schema_name text NOT NULL DEFAULT '',
  rule_id text NOT NULL,
  enabled boolean NOT NULL DEFAULT true,
  parameters jsonb NOT NULL DEFAULT '{}'::jsonb,
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (environment_id, schema_name, rule_id)
);

CREATE INDEX rule_policy_env_scope_idx ON rule_policy(environment_id, schema_name);

ALTER TABLE finding
  ADD COLUMN rule_id text NOT NULL DEFAULT '',
  ADD COLUMN rule_version text NOT NULL DEFAULT '',
  ADD COLUMN category text NOT NULL DEFAULT '',
  ADD COLUMN confidence numeric(4,3) NOT NULL DEFAULT 0,
  ADD COLUMN impact text NOT NULL DEFAULT '',
  ADD COLUMN risk text NOT NULL DEFAULT '',
  ADD COLUMN recommendation text NOT NULL DEFAULT '',
  ADD COLUMN validation text NOT NULL DEFAULT '',
  ADD COLUMN reference_urls jsonb NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN rule_parameters jsonb NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN recurrence_count integer NOT NULL DEFAULT 0,
  ADD COLUMN suppression_reason text,
  ADD COLUMN suppressed_until timestamptz,
  ADD COLUMN superseded_by uuid REFERENCES finding(id);

ALTER TABLE column_snapshot ADD COLUMN column_comment text;

CREATE TABLE rls_policy_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
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

CREATE TABLE grant_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  schema_name text NOT NULL,
  table_name text NOT NULL,
  grantee text NOT NULL,
  privileges text[] NOT NULL,
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name, schema_name, table_name, grantee)
);

CREATE TABLE object_dependency_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  source_schema text NOT NULL,
  source_name text NOT NULL,
  source_kind text NOT NULL,
  target_schema text NOT NULL,
  target_name text NOT NULL,
  target_kind text NOT NULL,
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name, source_schema, source_name, source_kind,
          target_schema, target_name, target_kind)
);

CREATE INDEX idx_assessment_table_scope
  ON table_snapshot (environment_id, audit_run_id, database_name, schema_name, table_name);
CREATE INDEX idx_assessment_fk_target
  ON constraint_snapshot (environment_id, audit_run_id, database_name, referenced_schema_name, referenced_table_name)
  WHERE constraint_type = 'f';
CREATE INDEX idx_assessment_findings
  ON finding (environment_id, audit_run_id, database_name, schema_name, object_name);
CREATE INDEX idx_assessment_grants
  ON grant_snapshot (environment_id, audit_run_id, database_name, schema_name, table_name);
CREATE INDEX idx_assessment_dependencies
  ON object_dependency_snapshot (environment_id, audit_run_id, database_name, target_schema, target_name);
CREATE INDEX idx_assessment_dependencies_source
  ON object_dependency_snapshot (environment_id, audit_run_id, database_name, source_schema, source_name);
CREATE INDEX idx_assessment_rls_policies
  ON rls_policy_snapshot (environment_id, audit_run_id, database_name, schema_name, table_name);

CREATE TABLE audit_baseline (
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL DEFAULT '',
  schema_name text NOT NULL DEFAULT '',
  table_name text NOT NULL DEFAULT '',
  audit_run_id uuid NOT NULL REFERENCES audit_run(id),
  selected_at timestamptz NOT NULL DEFAULT now(),
  selected_by text NOT NULL DEFAULT 'local',
  PRIMARY KEY (environment_id, database_name, schema_name, table_name)
);

CREATE TABLE baseline_selection_event (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL DEFAULT '',
  schema_name text NOT NULL DEFAULT '',
  table_name text NOT NULL DEFAULT '',
  previous_run_id uuid REFERENCES audit_run(id),
  selected_run_id uuid NOT NULL REFERENCES audit_run(id),
  selected_by text NOT NULL DEFAULT 'local',
  selected_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE baseline_comparison (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL DEFAULT '',
  schema_name text NOT NULL DEFAULT '',
  table_name text NOT NULL DEFAULT '',
  baseline_run_id uuid NOT NULL REFERENCES audit_run(id),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  status text NOT NULL CHECK (status IN ('complete','partial','incompatible')),
  added_tables integer NOT NULL DEFAULT 0,
  removed_tables integer NOT NULL DEFAULT 0,
  changed_tables integer NOT NULL DEFAULT 0,
  compared_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name, schema_name, table_name)
);

CREATE TABLE finding_event (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  finding_id uuid NOT NULL REFERENCES finding(id) ON DELETE CASCADE,
  audit_run_id uuid REFERENCES audit_run(id) ON DELETE CASCADE,
  event_type text NOT NULL CHECK (event_type IN ('observed','resolved','reopened','suppressed','acknowledged','superseded')),
  reason text NOT NULL DEFAULT '',
  category text NOT NULL DEFAULT '',
  severity text NOT NULL DEFAULT '',
  database_name text NOT NULL DEFAULT '',
  schema_name text NOT NULL DEFAULT '',
  object_name text NOT NULL DEFAULT '',
  rule_version text NOT NULL DEFAULT '',
  title text NOT NULL DEFAULT '',
  summary text NOT NULL DEFAULT '',
  recommendation text NOT NULL DEFAULT '',
  finding_status text NOT NULL DEFAULT 'open',
  impact text NOT NULL DEFAULT '',
  risk text NOT NULL DEFAULT '',
  validation text NOT NULL DEFAULT '',
  reference_urls jsonb NOT NULL DEFAULT '[]'::jsonb,
  rule_parameters jsonb NOT NULL DEFAULT '{}'::jsonb,
  confidence numeric(4,3) NOT NULL DEFAULT 0,
  evidence jsonb NOT NULL DEFAULT '{}'::jsonb,
  recorded_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (finding_id, audit_run_id, event_type)
);
CREATE INDEX finding_event_timeline_idx ON finding_event (finding_id, recorded_at, id);

CREATE TABLE report_job (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id),
  report_type text NOT NULL CHECK (report_type IN ('executive','technical','table')),
  filters jsonb NOT NULL DEFAULT '{}'::jsonb,
  requested_by text NOT NULL DEFAULT 'local',
  rule_version text NOT NULL DEFAULT '',
  status text NOT NULL CHECK (status IN ('queued','running','success','failed','cancelled')) DEFAULT 'queued',
  attempts integer NOT NULL DEFAULT 0,
  idempotency_key text NOT NULL,
  error text,
  created_at timestamptz NOT NULL DEFAULT now(),
  started_at timestamptz,
  finished_at timestamptz,
  expires_at timestamptz NOT NULL DEFAULT (now() + interval '30 days'),
  UNIQUE (environment_id, idempotency_key)
);
CREATE INDEX report_job_queue_idx ON report_job (status, created_at);
CREATE INDEX report_job_env_created_idx ON report_job (environment_id, created_at DESC);

CREATE TABLE report_artifact (
  report_job_id uuid PRIMARY KEY REFERENCES report_job(id) ON DELETE CASCADE,
  content bytea NOT NULL,
  sha256 text NOT NULL,
  size_bytes bigint NOT NULL,
  filename text NOT NULL,
  content_type text NOT NULL CHECK (content_type='application/pdf'),
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE auditor_user (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  username text NOT NULL UNIQUE CHECK (username ~ '^[a-zA-Z0-9_.-]{3,64}$'),
  password_hash text NOT NULL,
  role text NOT NULL CHECK (role IN ('viewer', 'auditor', 'operator')),
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE auditor_user_environment (
  user_id uuid NOT NULL REFERENCES auditor_user(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id) ON DELETE CASCADE,
  PRIMARY KEY (user_id, environment_id)
);

CREATE TABLE auditor_session (
  token_hash bytea PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES auditor_user(id) ON DELETE CASCADE,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX auditor_session_user_idx ON auditor_session (user_id);
CREATE INDEX auditor_session_expiry_idx ON auditor_session (expires_at);

CREATE TABLE auditor_operation_log (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  user_id uuid REFERENCES auditor_user(id) ON DELETE SET NULL,
  username text NOT NULL,
  action text NOT NULL,
  environment_id uuid REFERENCES audit_environment(id) ON DELETE SET NULL,
  resource_id text,
  result text NOT NULL CHECK (result IN ('success', 'denied', 'failed')),
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX auditor_operation_log_created_idx ON auditor_operation_log (created_at DESC);
CREATE INDEX auditor_operation_log_env_created_idx ON auditor_operation_log (environment_id, created_at DESC);

CREATE INDEX table_snapshot_page_idx ON table_snapshot (environment_id,audit_run_id,database_name,schema_name,table_name,id);
CREATE INDEX column_snapshot_page_idx ON column_snapshot (environment_id,audit_run_id,database_name,schema_name,table_name,ordinal_position,id);
CREATE INDEX index_snapshot_page_idx ON index_snapshot (environment_id,audit_run_id,database_name,schema_name,index_name,id);
CREATE INDEX view_snapshot_page_idx ON view_snapshot (environment_id,audit_run_id,database_name,schema_name,view_name,id);
CREATE INDEX function_snapshot_page_idx ON function_snapshot (environment_id,audit_run_id,database_name,schema_name,function_name,id);
CREATE INDEX audit_run_latest_idx ON audit_run (environment_id,started_at DESC,id DESC) WHERE status IN ('success','partial_success');

DROP INDEX finding_dedup_idx;
CREATE UNIQUE INDEX finding_dedup_version_idx ON finding (environment_id, dedup_key, rule_version);

CREATE TABLE audit_schedule (
  environment_id uuid NOT NULL REFERENCES audit_environment(id) ON DELETE CASCADE,
  profile text NOT NULL CHECK (profile IN ('fast', 'daily', 'weekly', 'monthly')),
  enabled boolean NOT NULL DEFAULT true,
  next_run_at timestamptz NOT NULL,
  last_status text,
  last_run_at timestamptz,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (environment_id, profile)
);


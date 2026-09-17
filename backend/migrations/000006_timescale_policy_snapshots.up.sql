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

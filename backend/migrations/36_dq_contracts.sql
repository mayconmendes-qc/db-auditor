-- Plan 03: versioned data-quality contracts (ADR-0001 D4/D13).
-- Counts and proportions only; never row values or freshness timestamps.

CREATE TABLE IF NOT EXISTS quality_contract (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  environment_id uuid NOT NULL REFERENCES audit_environment(id) ON DELETE CASCADE,
  database_name text NOT NULL CHECK (char_length(database_name) BETWEEN 1 AND 63),
  schema_name text NOT NULL CHECK (char_length(schema_name) BETWEEN 1 AND 63),
  table_name text NOT NULL CHECK (char_length(table_name) BETWEEN 1 AND 63),
  created_by text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (environment_id, database_name, schema_name, table_name)
);
CREATE INDEX IF NOT EXISTS quality_contract_env_idx
  ON quality_contract (environment_id, created_at DESC);

CREATE TABLE IF NOT EXISTS quality_contract_version (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  contract_id uuid NOT NULL REFERENCES quality_contract(id) ON DELETE CASCADE,
  version_number integer NOT NULL CHECK (version_number >= 1),
  document jsonb NOT NULL,
  source_yaml text NOT NULL DEFAULT '',
  sample_limit integer NOT NULL CHECK (sample_limit BETWEEN 1 AND 100000),
  schedule_profile text NOT NULL DEFAULT '' CHECK (
    schedule_profile = '' OR schedule_profile IN ('fast', 'daily', 'weekly', 'monthly')
  ),
  status text NOT NULL CHECK (status IN ('proposed', 'approved', 'superseded')),
  authored_by text NOT NULL DEFAULT '',
  authored_at timestamptz NOT NULL DEFAULT now(),
  approved_by text,
  approved_at timestamptz,
  UNIQUE (contract_id, version_number)
);
CREATE INDEX IF NOT EXISTS quality_contract_version_contract_idx
  ON quality_contract_version (contract_id, version_number DESC);
CREATE INDEX IF NOT EXISTS quality_contract_version_status_idx
  ON quality_contract_version (status) WHERE status = 'approved';

CREATE TABLE IF NOT EXISTS quality_contract_run (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  environment_id uuid NOT NULL REFERENCES audit_environment(id) ON DELETE CASCADE,
  contract_id uuid REFERENCES quality_contract(id) ON DELETE SET NULL,
  version_id uuid REFERENCES quality_contract_version(id) ON DELETE SET NULL,
  mode text NOT NULL CHECK (mode IN ('schedule', 'on_demand', 'cli')),
  status text NOT NULL DEFAULT 'running' CHECK (
    status IN ('running', 'success', 'partial_success', 'failed', 'skipped')
  ),
  coverage text NOT NULL DEFAULT 'complete' CHECK (
    coverage IN ('complete', 'partial', 'none')
  ),
  skip_reason text NOT NULL DEFAULT '',
  sample_limit integer NOT NULL DEFAULT 10000,
  token_id uuid REFERENCES environment_api_token(id) ON DELETE SET NULL,
  audit_run_id uuid REFERENCES audit_run(id) ON DELETE SET NULL,
  actor text NOT NULL DEFAULT '',
  started_at timestamptz NOT NULL DEFAULT now(),
  finished_at timestamptz,
  document jsonb NOT NULL DEFAULT '{}'::jsonb
);
CREATE INDEX IF NOT EXISTS quality_contract_run_env_idx
  ON quality_contract_run (environment_id, started_at DESC);
CREATE INDEX IF NOT EXISTS quality_contract_run_contract_idx
  ON quality_contract_run (contract_id, started_at DESC);

CREATE TABLE IF NOT EXISTS quality_contract_check_result (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  run_id uuid NOT NULL REFERENCES quality_contract_run(id) ON DELETE CASCADE,
  check_key text NOT NULL,
  check_type text NOT NULL CHECK (
    check_type IN ('nulls', 'uniqueness', 'orphan', 'domain', 'freshness', 'volume')
  ),
  column_name text NOT NULL DEFAULT '',
  result text NOT NULL CHECK (result IN ('passed', 'violated', 'not_executed')),
  reason text NOT NULL DEFAULT '',
  severity text NOT NULL DEFAULT 'medium',
  observed_count integer,
  observed_pct double precision,
  sample_size integer,
  sample_method text NOT NULL DEFAULT '',
  freshness_age_hours double precision,
  evidence jsonb NOT NULL DEFAULT '{}'::jsonb,
  UNIQUE (run_id, check_key)
);
CREATE INDEX IF NOT EXISTS quality_contract_check_result_run_idx
  ON quality_contract_check_result (run_id);

-- Additive migration for Sprint 15 residuals and Sprint 16 rule engine.
ALTER TABLE workload_snapshot
  ADD COLUMN IF NOT EXISTS extension_version text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS query_kind text NOT NULL DEFAULT 'other';

ALTER TABLE index_snapshot
  ADD COLUMN IF NOT EXISTS is_valid boolean NOT NULL DEFAULT true,
  ADD COLUMN IF NOT EXISTS is_ready boolean NOT NULL DEFAULT true,
  ADD COLUMN IF NOT EXISTS key_columns text[] NOT NULL DEFAULT '{}',
  ADD COLUMN IF NOT EXISTS predicate text NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS rule_catalog (
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

CREATE TABLE IF NOT EXISTS rule_policy (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  schema_name text NOT NULL DEFAULT '',
  rule_id text NOT NULL,
  enabled boolean NOT NULL DEFAULT true,
  parameters jsonb NOT NULL DEFAULT '{}'::jsonb,
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (environment_id, schema_name, rule_id)
);

CREATE INDEX IF NOT EXISTS rule_policy_env_scope_idx ON rule_policy(environment_id, schema_name);

ALTER TABLE finding
  ADD COLUMN IF NOT EXISTS rule_id text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS rule_version text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS category text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS confidence numeric(4,3) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS impact text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS risk text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS recommendation text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS validation text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS reference_urls jsonb NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN IF NOT EXISTS rule_parameters jsonb NOT NULL DEFAULT '{}'::jsonb;

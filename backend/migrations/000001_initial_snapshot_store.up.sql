CREATE EXTENSION IF NOT EXISTS pgcrypto;

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

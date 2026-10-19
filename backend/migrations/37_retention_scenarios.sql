-- Timescale retention advisor (spec 09): saved what-if scenarios per environment (ADR-0001 D9/D11).
-- Idempotent: CREATE ... IF NOT EXISTS only.

CREATE TABLE IF NOT EXISTS retention_scenario (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  environment_id uuid NOT NULL REFERENCES audit_environment(id) ON DELETE CASCADE,
  database_name text NOT NULL,
  schema_name text NOT NULL,
  hypertable_name text NOT NULL,
  label text NOT NULL DEFAULT '',
  retention_days integer NOT NULL DEFAULT 0 CHECK (retention_days >= 0),
  compress_after_days integer NOT NULL DEFAULT 0 CHECK (compress_after_days >= 0),
  -- At least one of retention / compression must be set (validated in API; DB allows 0,0 only if rejected upstream).
  audit_run_id uuid REFERENCES audit_run(id) ON DELETE SET NULL,
  current_bytes bigint NOT NULL DEFAULT 0,
  bytes_after bigint NOT NULL DEFAULT 0,
  bytes_saved bigint NOT NULL DEFAULT 0,
  gb_saved numeric NOT NULL DEFAULT 0,
  currency text NOT NULL DEFAULT '' CHECK (currency = '' OR currency IN ('BRL', 'USD')),
  money_saved numeric,
  price_version_id uuid REFERENCES cost_price_version(id) ON DELETE SET NULL,
  price_effective_from timestamptz,
  risks jsonb NOT NULL DEFAULT '[]'::jsonb,
  ratio_confidence_low boolean NOT NULL DEFAULT false,
  observed_ratio numeric,
  partial boolean NOT NULL DEFAULT false,
  saved_by text NOT NULL DEFAULT '',
  saved_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT retention_scenario_has_action CHECK (retention_days > 0 OR compress_after_days > 0)
);

CREATE INDEX IF NOT EXISTS retention_scenario_env_saved_idx
  ON retention_scenario (environment_id, saved_at DESC);

CREATE INDEX IF NOT EXISTS retention_scenario_ht_idx
  ON retention_scenario (environment_id, database_name, schema_name, hypertable_name);

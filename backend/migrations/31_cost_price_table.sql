-- Cost lens (spec 02): versioned price table per environment (ADR-0001 D3).
-- Idempotent: CREATE ... IF NOT EXISTS only. Currency locked after first version.

CREATE TABLE IF NOT EXISTS cost_price_version (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  environment_id uuid NOT NULL REFERENCES audit_environment(id) ON DELETE CASCADE,
  version integer NOT NULL,
  currency text NOT NULL CHECK (currency IN ('BRL', 'USD')),
  storage_gb_month numeric NOT NULL CHECK (storage_gb_month >= 0),
  -- I/O: either per million requests or per GB read (nullable unused side).
  io_per_million numeric CHECK (io_per_million IS NULL OR io_per_million >= 0),
  io_per_gb_read numeric CHECK (io_per_gb_read IS NULL OR io_per_gb_read >= 0),
  -- Compute: either per vCPU-hour or flat monthly instance cost.
  compute_vcpu_hour numeric CHECK (compute_vcpu_hour IS NULL OR compute_vcpu_hour >= 0),
  instance_month numeric CHECK (instance_month IS NULL OR instance_month >= 0),
  model_origin text NOT NULL DEFAULT 'custom'
    CHECK (model_origin IN ('rds_postgresql', 'cloud_sql', 'self_hosted', 'custom')),
  effective_from timestamptz NOT NULL,
  created_by text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (environment_id, version)
);

CREATE INDEX IF NOT EXISTS cost_price_version_env_eff_idx
  ON cost_price_version (environment_id, effective_from DESC, version DESC);

-- Lock currency to the first version of the environment (ADR-0001 D3).
CREATE OR REPLACE FUNCTION cost_price_version_currency_lock()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  locked text;
BEGIN
  SELECT currency INTO locked
  FROM cost_price_version
  WHERE environment_id = NEW.environment_id
  ORDER BY version ASC
  LIMIT 1;
  IF locked IS NOT NULL AND NEW.currency IS DISTINCT FROM locked THEN
    RAISE EXCEPTION 'currency locked to % for environment %', locked, NEW.environment_id
      USING ERRCODE = 'check_violation';
  END IF;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS cost_price_version_currency_lock_trg ON cost_price_version;
CREATE TRIGGER cost_price_version_currency_lock_trg
  BEFORE INSERT ON cost_price_version
  FOR EACH ROW
  EXECUTE FUNCTION cost_price_version_currency_lock();

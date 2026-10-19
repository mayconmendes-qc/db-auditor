-- Cost lens (spec 02): per-complete-run cost estimate totals for RF-09 series.
-- Only complete collections feed this table; partial runs are gaps.

CREATE TABLE IF NOT EXISTS cost_estimate_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  environment_id uuid NOT NULL REFERENCES audit_environment(id) ON DELETE CASCADE,
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  price_version_id uuid REFERENCES cost_price_version(id) ON DELETE SET NULL,
  currency text NOT NULL DEFAULT '' CHECK (currency = '' OR currency IN ('BRL', 'USD')),
  storage_month numeric NOT NULL DEFAULT 0 CHECK (storage_month >= 0),
  unused_index_month numeric NOT NULL DEFAULT 0 CHECK (unused_index_month >= 0),
  bloat_month numeric NOT NULL DEFAULT 0 CHECK (bloat_month >= 0),
  query_month numeric NOT NULL DEFAULT 0 CHECK (query_month >= 0),
  priced boolean NOT NULL DEFAULT false,
  confidence text NOT NULL DEFAULT 'medium'
    CHECK (confidence IN ('low', 'medium', 'high', 'none')),
  reason text NOT NULL DEFAULT '',
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (environment_id, audit_run_id)
);

CREATE INDEX IF NOT EXISTS cost_estimate_snapshot_env_collected_idx
  ON cost_estimate_snapshot (environment_id, collected_at DESC);

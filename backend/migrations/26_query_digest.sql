-- Query digest (spec 06): windowed workload comparison, history and per-environment destinations.
-- Idempotent: CREATE ... IF NOT EXISTS / ADD COLUMN IF NOT EXISTS only.

CREATE TABLE IF NOT EXISTS query_digest_config (
  environment_id uuid PRIMARY KEY REFERENCES audit_environment(id) ON DELETE CASCADE,
  window_days integer NOT NULL DEFAULT 7 CHECK (window_days BETWEEN 1 AND 90),
  thresholds jsonb NOT NULL DEFAULT '{}'::jsonb,
  delivery_enabled boolean NOT NULL DEFAULT true,
  updated_at timestamptz NOT NULL DEFAULT now(),
  updated_by text NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS query_digest (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  environment_id uuid NOT NULL REFERENCES audit_environment(id) ON DELETE CASCADE,
  state text NOT NULL CHECK (state IN ('ok', 'no_comparable_window', 'not_applicable')),
  reason text NOT NULL DEFAULT '',
  window_days integer NOT NULL DEFAULT 7,
  prior_from timestamptz,
  prior_to timestamptz,
  current_from timestamptz,
  current_to timestamptz,
  prior_first_run_id uuid,
  prior_last_run_id uuid,
  current_first_run_id uuid,
  current_last_run_id uuid,
  workload_limit_n integer NOT NULL DEFAULT 500,
  subwindow_note text NOT NULL DEFAULT '',
  coverage_notes text[] NOT NULL DEFAULT '{}',
  calculation_version text NOT NULL DEFAULT '1',
  markdown text NOT NULL DEFAULT '',
  regression_count integer NOT NULL DEFAULT 0,
  new_count integer NOT NULL DEFAULT 0,
  gone_count integer NOT NULL DEFAULT 0,
  improvement_count integer NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS query_digest_env_created_idx
  ON query_digest (environment_id, created_at DESC);

CREATE TABLE IF NOT EXISTS query_digest_item (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  digest_id uuid NOT NULL REFERENCES query_digest(id) ON DELETE CASCADE,
  classification text NOT NULL CHECK (classification IN ('regressao', 'nova', 'sumiu', 'melhora', 'estavel')),
  severity text NOT NULL DEFAULT 'informativa',
  fingerprint text NOT NULL,
  query_id text NOT NULL DEFAULT '',
  database_name text NOT NULL DEFAULT '',
  referenced_objects text[] NOT NULL DEFAULT '{}',
  prior_calls bigint NOT NULL DEFAULT 0,
  current_calls bigint NOT NULL DEFAULT 0,
  prior_mean_ms double precision NOT NULL DEFAULT 0,
  current_mean_ms double precision NOT NULL DEFAULT 0,
  prior_total_ms double precision NOT NULL DEFAULT 0,
  current_total_ms double precision NOT NULL DEFAULT 0,
  mean_change_pct double precision NOT NULL DEFAULT 0,
  total_change_pct double precision NOT NULL DEFAULT 0,
  prior_share double precision NOT NULL DEFAULT 0,
  current_share double precision NOT NULL DEFAULT 0,
  finding_id uuid,
  regression_alert_id uuid,
  related_finding_ids uuid[] NOT NULL DEFAULT '{}',
  sort_rank integer NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS query_digest_item_digest_idx
  ON query_digest_item (digest_id, classification, sort_rank);

CREATE TABLE IF NOT EXISTS notification_destination (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  environment_id uuid NOT NULL REFERENCES audit_environment(id) ON DELETE CASCADE,
  channel text NOT NULL CHECK (channel IN ('email', 'webhook')),
  -- email address or webhook URL (secret for webhook; never returned in full)
  target text NOT NULL,
  format text NOT NULL DEFAULT 'generic' CHECK (format IN ('generic', 'slack')),
  active boolean NOT NULL DEFAULT true,
  created_by text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS notification_destination_env_idx
  ON notification_destination (environment_id, active);

CREATE TABLE IF NOT EXISTS query_digest_delivery (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  digest_id uuid NOT NULL REFERENCES query_digest(id) ON DELETE CASCADE,
  destination_id uuid REFERENCES notification_destination(id) ON DELETE SET NULL,
  channel text NOT NULL DEFAULT '', -- email|webhook|global_webhook|global_email
  target_masked text NOT NULL DEFAULT '',
  status text NOT NULL CHECK (status IN ('pending', 'sent', 'failed')),
  attempt integer NOT NULL DEFAULT 1,
  error_sanitized text NOT NULL DEFAULT '',
  attempted_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS query_digest_delivery_digest_idx
  ON query_digest_delivery (digest_id, attempted_at DESC);

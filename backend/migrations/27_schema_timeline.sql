-- Schema change timeline (spec 04): events recorded at ingestion, production flag.
-- Idempotent.

ALTER TABLE audit_environment
  ADD COLUMN IF NOT EXISTS is_production boolean NOT NULL DEFAULT false;

CREATE TABLE IF NOT EXISTS schema_change_event (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  environment_id uuid NOT NULL REFERENCES audit_environment(id) ON DELETE CASCADE,
  prior_audit_run_id uuid REFERENCES audit_run(id) ON DELETE SET NULL,
  current_audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  detected_at timestamptz NOT NULL DEFAULT now(),
  object_type text NOT NULL,
  object_key text NOT NULL,
  change_kind text NOT NULL CHECK (change_kind IN ('added', 'removed', 'modified')),
  -- Fingerprints only — never raw SQL definitions.
  prior_fingerprint text NOT NULL DEFAULT '',
  current_fingerprint text NOT NULL DEFAULT '',
  changed_fields text[] NOT NULL DEFAULT '{}',
  correlation text NOT NULL DEFAULT 'none'
    CHECK (correlation IN ('deployment', 'migration_registry', 'none', 'not_verified')),
  annotation_id uuid,
  registry_version text NOT NULL DEFAULT '',
  confidence text NOT NULL DEFAULT 'high'
);

CREATE INDEX IF NOT EXISTS schema_change_event_env_detected_idx
  ON schema_change_event (environment_id, detected_at DESC, id);

CREATE TABLE IF NOT EXISTS schema_timeline_gap (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  environment_id uuid NOT NULL REFERENCES audit_environment(id) ON DELETE CASCADE,
  prior_audit_run_id uuid,
  current_audit_run_id uuid,
  gap_from timestamptz NOT NULL,
  gap_to timestamptz NOT NULL,
  reason text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS schema_timeline_gap_env_idx
  ON schema_timeline_gap (environment_id, gap_to DESC);

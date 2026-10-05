-- Persistent audit schedules. Safe to run on a volume that already has the baseline.
CREATE TABLE IF NOT EXISTS audit_schedule (
  environment_id uuid NOT NULL REFERENCES audit_environment(id) ON DELETE CASCADE,
  profile text NOT NULL CHECK (profile IN ('fast', 'daily', 'weekly', 'monthly')),
  enabled boolean NOT NULL DEFAULT true,
  next_run_at timestamptz NOT NULL,
  last_status text,
  last_run_at timestamptz,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (environment_id, profile)
);

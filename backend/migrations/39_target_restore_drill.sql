-- Target restore drills for audited databases (plan 08 / ADR-0001 D8).
CREATE TABLE IF NOT EXISTS target_restore_drill (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  kind text NOT NULL DEFAULT 'target_restore_drill' CHECK (kind = 'target_restore_drill'),
  artifact_sha256 text NOT NULL CHECK (artifact_sha256 ~ '^[0-9a-f]{64}$'),
  succeeded boolean NOT NULL,
  rto_seconds integer CHECK (rto_seconds IS NULL OR rto_seconds >= 0),
  prevalidate_seconds integer CHECK (prevalidate_seconds IS NULL OR prevalidate_seconds >= 0),
  verify_seconds integer CHECK (verify_seconds IS NULL OR verify_seconds >= 0),
  checks jsonb NOT NULL DEFAULT '[]'::jsonb,
  divergences jsonb NOT NULL DEFAULT '[]'::jsonb,
  cli_version text NOT NULL DEFAULT '',
  note text NOT NULL DEFAULT '',
  token_id uuid,
  performed_at timestamptz NOT NULL,
  recorded_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS target_restore_drill_env_performed_idx ON target_restore_drill (environment_id, performed_at DESC);
CREATE TABLE IF NOT EXISTS target_restore_policy (
  environment_id uuid PRIMARY KEY REFERENCES audit_environment(id),
  rto_target_seconds integer NOT NULL DEFAULT 600 CHECK (rto_target_seconds > 0),
  max_age_days integer NOT NULL DEFAULT 30 CHECK (max_age_days > 0),
  updated_by text NOT NULL DEFAULT '',
  updated_at timestamptz NOT NULL DEFAULT now()
);

-- Migration analysis history per environment (RF-10). No raw SQL or literals.

CREATE TABLE IF NOT EXISTS migration_analysis (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  environment_id uuid NOT NULL REFERENCES audit_environment(id) ON DELETE CASCADE,
  repository text NOT NULL DEFAULT '' CHECK (char_length(repository) <= 200),
  pr text NOT NULL DEFAULT '' CHECK (char_length(pr) <= 32),
  commit_sha text NOT NULL DEFAULT '' CHECK (char_length(commit_sha) <= 64),
  highest_severity text NOT NULL CHECK (highest_severity IN ('info','low','medium','high','critical')),
  rules_version text NOT NULL CHECK (char_length(rules_version) <= 32),
  pg_version integer NOT NULL CHECK (pg_version BETWEEN 14 AND 18),
  context_mode text NOT NULL CHECK (context_mode IN ('none','snapshot','direct')),
  audit_run_id uuid REFERENCES audit_run(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  created_by_token uuid REFERENCES environment_api_token(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS migration_analysis_env_created_idx
  ON migration_analysis (environment_id, created_at DESC);

CREATE TABLE IF NOT EXISTS migration_analysis_item (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  analysis_id uuid NOT NULL REFERENCES migration_analysis(id) ON DELETE CASCADE,
  file_path text NOT NULL CHECK (char_length(file_path) <= 512),
  line_no integer NOT NULL CHECK (line_no > 0),
  code text NOT NULL CHECK (char_length(code) <= 16),
  operation text NOT NULL CHECK (char_length(operation) <= 64),
  object_name text NOT NULL DEFAULT '' CHECK (char_length(object_name) <= 256),
  lock_mode text NOT NULL DEFAULT '' CHECK (char_length(lock_mode) <= 64),
  rewrite_or_scan text NOT NULL DEFAULT '' CHECK (char_length(rewrite_or_scan) <= 128),
  size_bytes bigint,
  duration_estimate text NOT NULL DEFAULT '' CHECK (char_length(duration_estimate) <= 64),
  severity text NOT NULL CHECK (severity IN ('info','low','medium','high','critical')),
  confidence text NOT NULL CHECK (confidence IN ('low','medium','high')),
  suppressed boolean NOT NULL DEFAULT false,
  justification text NOT NULL DEFAULT '' CHECK (char_length(justification) <= 500)
);

CREATE INDEX IF NOT EXISTS migration_analysis_item_analysis_idx
  ON migration_analysis_item (analysis_id);

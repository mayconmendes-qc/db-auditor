-- Third-stage diagnostics and measured outcomes live only in the control DB.
-- Existing audit history is retained; this migration never touches targets.
-- Statements are idempotent so a partial apply can be retried safely.
ALTER TABLE audit_environment ADD COLUMN IF NOT EXISTS engine text NOT NULL DEFAULT 'postgresql';

DO $$
BEGIN
  ALTER TABLE audit_environment
    ADD CONSTRAINT audit_environment_engine_format
    CHECK (engine ~ '^[a-z][a-z0-9_]{1,31}$');
EXCEPTION
  WHEN duplicate_object THEN
    NULL;
END
$$;

CREATE TABLE IF NOT EXISTS quality_scan (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  schema_name text NOT NULL,
  table_name text NOT NULL,
  sample_limit integer NOT NULL CHECK (sample_limit BETWEEN 1 AND 1000),
  sampled_rows integer NOT NULL CHECK (sampled_rows BETWEEN 0 AND 1000),
  sample_method text NOT NULL DEFAULT 'bounded_first_rows',
  actor text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS quality_scan_environment_idx ON quality_scan(environment_id,created_at DESC);

CREATE TABLE IF NOT EXISTS quality_issue (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  scan_id uuid NOT NULL REFERENCES quality_scan(id),
  check_kind text NOT NULL CHECK (check_kind IN ('null','duplicate','orphan','date_range','distribution')),
  column_name text NOT NULL,
  affected_rows integer NOT NULL CHECK (affected_rows >= 0),
  sampled_rows integer NOT NULL CHECK (sampled_rows BETWEEN 0 AND 1000),
  status text NOT NULL DEFAULT 'suggested' CHECK (status IN ('suggested','in_review','planned','executed_externally','validated','discarded')),
  owner_name text NOT NULL DEFAULT '',
  justification text NOT NULL DEFAULT '',
  result_note text NOT NULL DEFAULT '',
  updated_by text NOT NULL DEFAULT '',
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS quality_issue_scan_idx ON quality_issue(scan_id);

CREATE TABLE IF NOT EXISTS quality_issue_event (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  issue_id uuid NOT NULL REFERENCES quality_issue(id),
  previous_status text NOT NULL,
  new_status text NOT NULL,
  actor text NOT NULL,
  owner_name text NOT NULL,
  justification text NOT NULL,
  result_note text NOT NULL,
  recorded_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS quality_issue_event_issue_idx ON quality_issue_event(issue_id,recorded_at DESC);

CREATE TABLE IF NOT EXISTS finding_action_measurement (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  finding_id uuid NOT NULL REFERENCES finding(id),
  before_run_id uuid NOT NULL REFERENCES audit_run(id),
  after_run_id uuid NOT NULL REFERENCES audit_run(id),
  metric text NOT NULL CHECK (metric IN ('table_size_bytes','finding_observed')),
  before_value bigint,
  after_value bigint,
  comparable boolean NOT NULL,
  comparison_note text NOT NULL,
  hypothesis text NOT NULL,
  window_note text NOT NULL,
  recorded_by text NOT NULL,
  recorded_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(finding_id,before_run_id,after_run_id,metric)
);
CREATE INDEX IF NOT EXISTS finding_action_measurement_finding_idx ON finding_action_measurement(finding_id,recorded_at DESC);

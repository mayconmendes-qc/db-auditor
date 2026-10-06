-- Workflow and annotations live only in the auditor's control database.
-- This migration does not write to any audited database.
CREATE TABLE IF NOT EXISTS finding_action (
  finding_id uuid PRIMARY KEY REFERENCES finding(id) ON DELETE CASCADE,
  status text NOT NULL DEFAULT 'suggested' CHECK (status IN
    ('suggested','in_review','planned','executed_externally','validated','discarded')),
  owner_name text NOT NULL DEFAULT '',
  justification text NOT NULL DEFAULT '',
  result_note text NOT NULL DEFAULT '',
  updated_by text NOT NULL DEFAULT '',
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS finding_action_status_idx ON finding_action (status,updated_at DESC);

CREATE TABLE IF NOT EXISTS finding_action_event (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  finding_id uuid NOT NULL REFERENCES finding(id) ON DELETE CASCADE,
  status text NOT NULL,
  owner_name text NOT NULL DEFAULT '',
  justification text NOT NULL DEFAULT '',
  result_note text NOT NULL DEFAULT '',
  actor text NOT NULL,
  recorded_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS finding_action_event_finding_idx ON finding_action_event (finding_id,recorded_at DESC);

CREATE TABLE IF NOT EXISTS audit_annotation (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  environment_id uuid NOT NULL REFERENCES audit_environment(id) ON DELETE CASCADE,
  audit_run_id uuid REFERENCES audit_run(id) ON DELETE SET NULL,
  kind text NOT NULL CHECK (kind IN ('deployment','maintenance','incident','note')),
  note text NOT NULL CHECK (length(note) BETWEEN 1 AND 2000),
  actor text NOT NULL,
  occurred_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS audit_annotation_env_time_idx ON audit_annotation (environment_id,occurred_at DESC);

CREATE TABLE IF NOT EXISTS regression_alert (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  environment_id uuid NOT NULL REFERENCES audit_environment(id) ON DELETE CASCADE,
  baseline_run_id uuid NOT NULL REFERENCES audit_run(id),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  category text NOT NULL CHECK (category IN ('structure','findings')),
  evidence jsonb NOT NULL DEFAULT '{}'::jsonb,
  status text NOT NULL DEFAULT 'open' CHECK (status IN ('open','acknowledged')),
  reason text NOT NULL DEFAULT '',
  acknowledged_by text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  acknowledged_at timestamptz,
  UNIQUE (environment_id,baseline_run_id,audit_run_id,category)
);
CREATE INDEX IF NOT EXISTS regression_alert_env_created_idx ON regression_alert (environment_id,created_at DESC);

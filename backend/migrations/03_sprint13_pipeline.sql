-- Sprint 13: coherent run coverage and automatic analysis lifecycle.
-- This is intentionally additive; previously applied migrations remain immutable.

ALTER TABLE index_snapshot ADD COLUMN stats_reset timestamptz;
ALTER TABLE table_snapshot ADD COLUMN stats_reset timestamptz;
ALTER TABLE job_snapshot ADD COLUMN last_run_status text;
ALTER TABLE job_snapshot ADD COLUMN total_failures bigint NOT NULL DEFAULT 0;
ALTER TABLE continuous_aggregate_snapshot ADD COLUMN view_definition text NOT NULL DEFAULT '';

CREATE TABLE audit_run_coverage (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  collector_name text NOT NULL,
  database_name text NOT NULL DEFAULT '',
  status text NOT NULL CHECK (status IN ('attempted', 'success', 'skipped', 'failed')),
  rows_collected bigint NOT NULL DEFAULT 0,
  warning text,
  error text,
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, collector_name, database_name)
);

CREATE INDEX audit_run_coverage_run_idx
  ON audit_run_coverage (audit_run_id, collector_name, database_name);

CREATE TABLE analysis_run (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  status text NOT NULL CHECK (status IN ('running', 'success', 'failed')),
  analyzer_version text NOT NULL,
  findings_produced integer NOT NULL DEFAULT 0,
  findings_saved integer NOT NULL DEFAULT 0,
  error text,
  started_at timestamptz NOT NULL DEFAULT now(),
  finished_at timestamptz,
  UNIQUE (audit_run_id)
);

CREATE INDEX analysis_run_environment_started_idx
  ON analysis_run (environment_id, started_at DESC);

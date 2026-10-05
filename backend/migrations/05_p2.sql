-- P2 schema. Idempotent. Do not edit 01_baseline.sql.

ALTER TABLE audit_environment ADD COLUMN IF NOT EXISTS expects_replica boolean NOT NULL DEFAULT false;

CREATE TABLE IF NOT EXISTS snapshot_backup (
  id smallint PRIMARY KEY DEFAULT 1 CHECK (id = 1),
  last_success_at timestamptz,
  last_attempt_at timestamptz,
  last_status text NOT NULL DEFAULT '',
  artifact_path text NOT NULL DEFAULT ''
);

INSERT INTO snapshot_backup (id) VALUES (1) ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS replication_snapshot (
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  replica_count integer NOT NULL DEFAULT 0,
  apply_lag_bytes bigint,
  flush_lag_bytes bigint,
  write_lag_bytes bigint,
  replay_lag_bytes bigint,
  archive_failed_count bigint NOT NULL DEFAULT 0,
  last_archived_time timestamptz,
  last_failed_time timestamptz,
  permission_ok boolean NOT NULL DEFAULT false,
  collected_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (audit_run_id, database_name)
);

CREATE TABLE IF NOT EXISTS scheduled_report (
  id uuid PRIMARY KEY,
  environment_id uuid NOT NULL REFERENCES audit_environment(id) ON DELETE CASCADE,
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  report_type text NOT NULL DEFAULT 'executive',
  status text NOT NULL DEFAULT 'queued',
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (environment_id, audit_run_id, report_type)
);

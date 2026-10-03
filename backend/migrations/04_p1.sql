-- P1 schema. Idempotent: the Postgres entrypoint and the API migrator can both apply it.
-- Do not edit 01_baseline.sql after it has been stamped; checksum drift refuses to boot.

ALTER TABLE finding ADD COLUMN IF NOT EXISTS assignee text NOT NULL DEFAULT '';
ALTER TABLE finding ADD COLUMN IF NOT EXISTS due_at timestamptz;

ALTER TABLE job_snapshot ADD COLUMN IF NOT EXISTS last_run_duration text NOT NULL DEFAULT '';
ALTER TABLE job_snapshot ADD COLUMN IF NOT EXISTS max_background_workers integer NOT NULL DEFAULT 0;

ALTER TABLE continuous_aggregate_snapshot ADD COLUMN IF NOT EXISTS lag_interval text NOT NULL DEFAULT '';

ALTER TABLE chunk_snapshot ADD COLUMN IF NOT EXISTS before_compression_bytes bigint NOT NULL DEFAULT 0;
ALTER TABLE chunk_snapshot ADD COLUMN IF NOT EXISTS after_compression_bytes bigint NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS alert_delivery (
  environment_id uuid NOT NULL REFERENCES audit_environment(id) ON DELETE CASCADE,
  dedup_key text NOT NULL,
  delivered_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (environment_id, dedup_key)
);

CREATE TABLE IF NOT EXISTS auditor_privilege_snapshot (
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  role_name text NOT NULL DEFAULT '',
  is_superuser boolean NOT NULL DEFAULT false,
  can_create_db boolean NOT NULL DEFAULT false,
  can_create_role boolean NOT NULL DEFAULT false,
  replication boolean NOT NULL DEFAULT false,
  can_write boolean NOT NULL DEFAULT false,
  check_failed boolean NOT NULL DEFAULT false,
  collected_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (audit_run_id, database_name)
);

CREATE TABLE IF NOT EXISTS server_setting_snapshot (
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  setting_name text NOT NULL,
  setting_value text NOT NULL DEFAULT '',
  collected_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (audit_run_id, database_name, setting_name)
);

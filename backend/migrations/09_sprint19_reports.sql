-- Sprint 19: durable asynchronous report jobs and bounded artifacts.
CREATE TABLE report_job (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id),
  report_type text NOT NULL CHECK (report_type IN ('executive','technical','table')),
  filters jsonb NOT NULL DEFAULT '{}'::jsonb,
  requested_by text NOT NULL DEFAULT 'local',
  rule_version text NOT NULL DEFAULT '',
  status text NOT NULL CHECK (status IN ('queued','running','success','failed','cancelled')) DEFAULT 'queued',
  attempts integer NOT NULL DEFAULT 0,
  idempotency_key text NOT NULL,
  error text,
  created_at timestamptz NOT NULL DEFAULT now(),
  started_at timestamptz,
  finished_at timestamptz,
  expires_at timestamptz NOT NULL DEFAULT (now() + interval '30 days'),
  UNIQUE (environment_id, idempotency_key)
);
CREATE INDEX report_job_queue_idx ON report_job (status, created_at);
CREATE INDEX report_job_env_created_idx ON report_job (environment_id, created_at DESC);

CREATE TABLE report_artifact (
  report_job_id uuid PRIMARY KEY REFERENCES report_job(id) ON DELETE CASCADE,
  content bytea NOT NULL,
  sha256 text NOT NULL,
  size_bytes bigint NOT NULL,
  filename text NOT NULL,
  content_type text NOT NULL CHECK (content_type='application/pdf'),
  created_at timestamptz NOT NULL DEFAULT now()
);

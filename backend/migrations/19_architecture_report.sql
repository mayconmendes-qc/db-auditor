-- Control database only. Target databases are never migrated by DB Auditor.
ALTER TABLE report_job DROP CONSTRAINT IF EXISTS report_job_report_type_check;
ALTER TABLE report_job ADD CONSTRAINT report_job_report_type_check
  CHECK (report_type IN ('executive','technical','table','architecture'));
ALTER TABLE report_artifact DROP CONSTRAINT IF EXISTS report_artifact_content_type_check;
ALTER TABLE report_artifact ADD CONSTRAINT report_artifact_content_type_check
  CHECK (content_type IN ('application/pdf','text/markdown; charset=utf-8'));

CREATE TABLE IF NOT EXISTS architecture_decision (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  object_ref text NOT NULL,
  subject text NOT NULL,
  value text NOT NULL,
  version integer NOT NULL,
  author text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (environment_id,database_name,object_ref,subject,version)
);
CREATE INDEX IF NOT EXISTS architecture_decision_scope_idx ON architecture_decision(environment_id,database_name,object_ref,subject,version DESC);
ALTER TABLE report_job ADD COLUMN IF NOT EXISTS architecture_decisions jsonb NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE report_job ADD COLUMN IF NOT EXISTS report_format text NOT NULL DEFAULT 'pdf'
  CHECK (report_format IN ('pdf','markdown'));
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='report_job'::regclass AND conname='report_job_format_matches_type') THEN
    ALTER TABLE report_job ADD CONSTRAINT report_job_format_matches_type
      CHECK ((report_type='architecture' AND report_format='markdown') OR
             (report_type<>'architecture' AND report_format='pdf'));
  END IF;
END $$;
ALTER TABLE report_job ADD COLUMN IF NOT EXISTS architecture_template_version text NOT NULL DEFAULT '';
ALTER TABLE workload_snapshot ADD COLUMN IF NOT EXISTS temp_blks_read bigint;
ALTER TABLE workload_snapshot ADD COLUMN IF NOT EXISTS temp_blks_written bigint;
ALTER TABLE workload_snapshot ADD COLUMN IF NOT EXISTS block_size_bytes integer;
ALTER TABLE constraint_snapshot ADD COLUMN IF NOT EXISTS supporting_index text;

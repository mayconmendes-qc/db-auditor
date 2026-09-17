CREATE TABLE finding (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  audit_run_id uuid REFERENCES audit_run(id),
  finding_type text NOT NULL,
  severity text NOT NULL CHECK (severity IN ('info', 'low', 'medium', 'high', 'critical')),
  status text NOT NULL CHECK (status IN ('open', 'acknowledged', 'resolved', 'suppressed')) DEFAULT 'open',
  title text NOT NULL,
  summary text NOT NULL DEFAULT '',
  object_type text NOT NULL DEFAULT '',
  object_key text NOT NULL DEFAULT '',
  database_name text NOT NULL DEFAULT '',
  schema_name text NOT NULL DEFAULT '',
  object_name text NOT NULL DEFAULT '',
  evidence jsonb NOT NULL DEFAULT '{}'::jsonb,
  dedup_key text NOT NULL,
  first_seen_at timestamptz NOT NULL DEFAULT now(),
  last_seen_at timestamptz NOT NULL DEFAULT now(),
  resolved_at timestamptz,
  notes text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX finding_dedup_idx ON finding (environment_id, dedup_key);
CREATE INDEX finding_env_status_idx ON finding (environment_id, status);
CREATE INDEX finding_type_idx ON finding (finding_type);
CREATE INDEX finding_severity_idx ON finding (severity);
CREATE INDEX finding_last_seen_idx ON finding (last_seen_at DESC);

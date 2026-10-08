-- Store only public role metadata; no password hashes or session details.
CREATE TABLE IF NOT EXISTS account_role_snapshot (
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  role_name text NOT NULL,
  can_login boolean NOT NULL,
  valid_until timestamptz,
  sampled_active boolean NOT NULL,
  collected_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (audit_run_id,database_name,role_name)
);
CREATE INDEX IF NOT EXISTS account_role_history_idx ON account_role_snapshot(environment_id,role_name,collected_at DESC);

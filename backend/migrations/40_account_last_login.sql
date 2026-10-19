-- Last-login aggregate from log-anomaly CLI (ADR-0001 D10 / plan 07 L6).
-- Only role name and latest login timestamp per environment; source is always 'log'.
CREATE TABLE IF NOT EXISTS account_last_login_observation (
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  role_name text NOT NULL CHECK (char_length(role_name) BETWEEN 1 AND 128),
  last_login_at timestamptz NOT NULL,
  source text NOT NULL DEFAULT 'log' CHECK (source = 'log'),
  token_id uuid,
  received_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (environment_id, role_name)
);
CREATE INDEX IF NOT EXISTS account_last_login_env_received_idx
  ON account_last_login_observation (environment_id, received_at DESC);

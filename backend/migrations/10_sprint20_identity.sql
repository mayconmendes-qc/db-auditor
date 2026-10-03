-- Sprint 20: local identity, revocable sessions and a durable operation trail.
CREATE TABLE auditor_user (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  username text NOT NULL UNIQUE CHECK (username ~ '^[a-zA-Z0-9_.-]{3,64}$'),
  password_hash text NOT NULL,
  role text NOT NULL CHECK (role IN ('viewer', 'auditor', 'operator')),
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE auditor_user_environment (
  user_id uuid NOT NULL REFERENCES auditor_user(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id) ON DELETE CASCADE,
  PRIMARY KEY (user_id, environment_id)
);

CREATE TABLE auditor_session (
  token_hash bytea PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES auditor_user(id) ON DELETE CASCADE,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX auditor_session_user_idx ON auditor_session (user_id);
CREATE INDEX auditor_session_expiry_idx ON auditor_session (expires_at);

CREATE TABLE auditor_operation_log (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  user_id uuid REFERENCES auditor_user(id) ON DELETE SET NULL,
  username text NOT NULL,
  action text NOT NULL,
  environment_id uuid REFERENCES audit_environment(id) ON DELETE SET NULL,
  resource_id text,
  result text NOT NULL CHECK (result IN ('success', 'denied', 'failed')),
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX auditor_operation_log_created_idx ON auditor_operation_log (created_at DESC);
CREATE INDEX auditor_operation_log_env_created_idx ON auditor_operation_log (environment_id, created_at DESC);

-- Stable ordered inventory pages and the latest coherent snapshot count.
CREATE INDEX table_snapshot_page_idx ON table_snapshot (environment_id,audit_run_id,database_name,schema_name,table_name,id);
CREATE INDEX column_snapshot_page_idx ON column_snapshot (environment_id,audit_run_id,database_name,schema_name,table_name,ordinal_position,id);
CREATE INDEX index_snapshot_page_idx ON index_snapshot (environment_id,audit_run_id,database_name,schema_name,index_name,id);
CREATE INDEX view_snapshot_page_idx ON view_snapshot (environment_id,audit_run_id,database_name,schema_name,view_name,id);
CREATE INDEX function_snapshot_page_idx ON function_snapshot (environment_id,audit_run_id,database_name,schema_name,function_name,id);
CREATE INDEX audit_run_latest_idx ON audit_run (environment_id,started_at DESC,id DESC) WHERE status IN ('success','partial_success');

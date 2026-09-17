CREATE TABLE constraint_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  schema_name text NOT NULL,
  table_name text NOT NULL,
  constraint_name text NOT NULL,
  constraint_type text NOT NULL,
  constraint_definition text NOT NULL,
  is_validated boolean NOT NULL DEFAULT true,
  is_deferrable boolean NOT NULL DEFAULT false,
  is_deferred boolean NOT NULL DEFAULT false,
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name, schema_name, constraint_name)
);

CREATE TABLE view_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  schema_name text NOT NULL,
  view_name text NOT NULL,
  owner_name text,
  relkind text NOT NULL,
  view_definition text NOT NULL,
  size_bytes bigint NOT NULL DEFAULT 0,
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name, schema_name, view_name)
);

CREATE TABLE function_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  schema_name text NOT NULL,
  function_name text NOT NULL,
  identity_arguments text NOT NULL DEFAULT '',
  owner_name text,
  language_name text,
  is_security_definer boolean NOT NULL DEFAULT false,
  volatility text,
  parallel_safety text,
  kind text,
  function_definition text,
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name, schema_name, function_name, identity_arguments)
);

CREATE TABLE extension_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  extension_name text NOT NULL,
  extension_version text,
  schema_name text,
  is_relocatable boolean NOT NULL DEFAULT false,
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name, extension_name)
);

CREATE INDEX constraint_snapshot_env_run_idx ON constraint_snapshot (environment_id, audit_run_id);
CREATE INDEX view_snapshot_env_run_idx ON view_snapshot (environment_id, audit_run_id);
CREATE INDEX function_snapshot_env_run_idx ON function_snapshot (environment_id, audit_run_id);
CREATE INDEX extension_snapshot_env_run_idx ON extension_snapshot (environment_id, audit_run_id);

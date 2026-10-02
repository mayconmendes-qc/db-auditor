-- Sprint 17: additive metadata and indexes; existing snapshots remain intact.
ALTER TABLE column_snapshot ADD COLUMN IF NOT EXISTS column_comment text;
ALTER TABLE table_snapshot ADD COLUMN IF NOT EXISTS storage_parameters text[] NOT NULL DEFAULT ARRAY[]::text[];

-- 04_sprint14_structural.sql used policy_snapshot, which already belongs to
-- TimescaleDB policies in the baseline. Keep both domains separate.
CREATE TABLE IF NOT EXISTS rls_policy_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  schema_name text NOT NULL,
  table_name text NOT NULL,
  policy_name text NOT NULL,
  permissive text,
  roles text[],
  cmd text,
  qual text,
  with_check text,
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name, schema_name, table_name, policy_name)
);

CREATE TABLE IF NOT EXISTS grant_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  schema_name text NOT NULL,
  table_name text NOT NULL,
  grantee text NOT NULL,
  privileges text[] NOT NULL,
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name, schema_name, table_name, grantee)
);

CREATE TABLE IF NOT EXISTS object_dependency_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  source_schema text NOT NULL,
  source_name text NOT NULL,
  source_kind text NOT NULL,
  target_schema text NOT NULL,
  target_name text NOT NULL,
  target_kind text NOT NULL,
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name, source_schema, source_name, source_kind,
          target_schema, target_name, target_kind)
);

CREATE INDEX IF NOT EXISTS idx_assessment_table_scope
  ON table_snapshot (environment_id, audit_run_id, database_name, schema_name, table_name);
CREATE INDEX IF NOT EXISTS idx_assessment_fk_target
  ON constraint_snapshot (environment_id, audit_run_id, database_name, referenced_schema_name, referenced_table_name)
  WHERE constraint_type = 'f';
CREATE INDEX IF NOT EXISTS idx_assessment_findings
  ON finding (environment_id, audit_run_id, database_name, schema_name, object_name);
CREATE INDEX IF NOT EXISTS idx_assessment_grants
  ON grant_snapshot (environment_id, audit_run_id, database_name, schema_name, table_name);
CREATE INDEX IF NOT EXISTS idx_assessment_dependencies
  ON object_dependency_snapshot (environment_id, audit_run_id, database_name, target_schema, target_name);
CREATE INDEX IF NOT EXISTS idx_assessment_dependencies_source
  ON object_dependency_snapshot (environment_id, audit_run_id, database_name, source_schema, source_name);
CREATE INDEX IF NOT EXISTS idx_assessment_rls_policies
  ON rls_policy_snapshot (environment_id, audit_run_id, database_name, schema_name, table_name);

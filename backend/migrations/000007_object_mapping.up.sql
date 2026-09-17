CREATE TABLE object_mapping (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  source_environment_id uuid NOT NULL REFERENCES audit_environment(id),
  target_environment_id uuid NOT NULL REFERENCES audit_environment(id),
  source_database text NOT NULL,
  source_schema text NOT NULL DEFAULT '',
  source_object_type text NOT NULL,
  source_object_name text NOT NULL,
  target_database text NOT NULL,
  target_schema text NOT NULL DEFAULT '',
  target_object_type text NOT NULL,
  target_object_name text NOT NULL,
  relation_type text NOT NULL CHECK (relation_type IN ('database_to_schema', 'identical', 'renamed', 'unmatched')),
  confidence numeric(4,3) NOT NULL DEFAULT 0,
  status text NOT NULL CHECK (status IN ('suggested', 'validated', 'rejected', 'manual')) DEFAULT 'manual',
  source_fingerprint text,
  target_fingerprint text,
  fingerprint_algorithm text,
  notes text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT object_mapping_envs_distinct CHECK (source_environment_id <> target_environment_id)
);

CREATE INDEX object_mapping_source_env_idx ON object_mapping (source_environment_id);
CREATE INDEX object_mapping_target_env_idx ON object_mapping (target_environment_id);
CREATE INDEX object_mapping_status_idx ON object_mapping (status);
CREATE UNIQUE INDEX object_mapping_unique_pair_idx ON object_mapping (
  source_environment_id, target_environment_id,
  source_database, source_schema, source_object_type, source_object_name,
  target_database, target_schema, target_object_type, target_object_name
);

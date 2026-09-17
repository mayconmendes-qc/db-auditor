CREATE TABLE timescale_version_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  extension_name text NOT NULL,
  extension_version text NOT NULL,
  schema_name text,
  major int NOT NULL DEFAULT 0,
  minor int NOT NULL DEFAULT 0,
  patch int NOT NULL DEFAULT 0,
  compatible boolean NOT NULL DEFAULT false,
  compatibility_note text,
  collection_status text NOT NULL DEFAULT 'OK',
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name)
);

CREATE TABLE hypertable_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  schema_name text NOT NULL,
  hypertable_name text NOT NULL,
  owner_name text,
  num_dimensions int NOT NULL DEFAULT 0,
  num_chunks int NOT NULL DEFAULT 0,
  compression_enabled boolean NOT NULL DEFAULT false,
  is_distributed boolean NOT NULL DEFAULT false,
  total_size_bytes bigint NOT NULL DEFAULT 0,
  data_size_bytes bigint NOT NULL DEFAULT 0,
  index_size_bytes bigint NOT NULL DEFAULT 0,
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name, schema_name, hypertable_name)
);

CREATE TABLE dimension_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  schema_name text NOT NULL,
  hypertable_name text NOT NULL,
  dimension_number int NOT NULL,
  column_name text NOT NULL,
  column_type text,
  dimension_type text,
  time_interval text,
  integer_interval text,
  integer_now_func text,
  num_slices int,
  partitioning_func text,
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name, schema_name, hypertable_name, dimension_number)
);

CREATE TABLE chunk_snapshot (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  audit_run_id uuid NOT NULL REFERENCES audit_run(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES audit_environment(id),
  database_name text NOT NULL,
  schema_name text NOT NULL,
  hypertable_name text NOT NULL,
  chunk_schema text NOT NULL,
  chunk_name text NOT NULL,
  range_start timestamptz,
  range_end timestamptz,
  range_start_integer bigint,
  range_end_integer bigint,
  is_compressed boolean NOT NULL DEFAULT false,
  chunk_tablespace text,
  total_size_bytes bigint NOT NULL DEFAULT 0,
  data_size_bytes bigint NOT NULL DEFAULT 0,
  index_size_bytes bigint NOT NULL DEFAULT 0,
  collected_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (audit_run_id, database_name, chunk_schema, chunk_name)
);

CREATE INDEX timescale_version_snapshot_env_run_idx ON timescale_version_snapshot (environment_id, audit_run_id);
CREATE INDEX hypertable_snapshot_env_run_idx ON hypertable_snapshot (environment_id, audit_run_id);
CREATE INDEX dimension_snapshot_env_run_idx ON dimension_snapshot (environment_id, audit_run_id);
CREATE INDEX chunk_snapshot_env_run_idx ON chunk_snapshot (environment_id, audit_run_id);
CREATE INDEX chunk_snapshot_ht_idx ON chunk_snapshot (audit_run_id, database_name, schema_name, hypertable_name);

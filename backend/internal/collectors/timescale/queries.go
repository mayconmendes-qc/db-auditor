package timescale

// SQL kept in sync with backend/sql/collectors/timescale/*.sql (source of truth for review).
// Optional / version-dependent columns are read via to_jsonb(row) so missing attributes
// become NULL instead of ERROR 42703 (Tiger Cloud / TimescaleDB ≥ 2.14).

const versionSQL = `
SELECT
  current_database() AS database_name,
  e.extname AS extension_name,
  e.extversion AS extension_version,
  n.nspname AS schema_name
FROM pg_extension e
JOIN pg_namespace n ON n.oid = e.extnamespace
WHERE e.extname = 'timescaledb'
`

// is_distributed was sunset in TimescaleDB v2.14 (multi-node removed).
const hypertablesSQL = `
SELECT
  current_database() AS database_name,
  h.hypertable_schema AS schema_name,
  h.hypertable_name AS hypertable_name,
  h.owner AS owner_name,
  COALESCE(h.num_dimensions, 0)::int AS num_dimensions,
  COALESCE(h.num_chunks, 0)::int AS num_chunks,
  COALESCE(h.compression_enabled, false) AS compression_enabled,
  COALESCE((to_jsonb(h)->>'is_distributed')::boolean, false) AS is_distributed,
  COALESCE(pg_total_relation_size(format('%I.%I', h.hypertable_schema, h.hypertable_name)::regclass), 0) AS total_size_bytes,
  COALESCE(pg_relation_size(format('%I.%I', h.hypertable_schema, h.hypertable_name)::regclass), 0) AS data_size_bytes,
  COALESCE(pg_indexes_size(format('%I.%I', h.hypertable_schema, h.hypertable_name)::regclass), 0) AS index_size_bytes
FROM timescaledb_information.hypertables h
ORDER BY h.hypertable_schema, h.hypertable_name
`

// num_slices renamed to num_partitions on newer informational views.
const dimensionsSQL = `
SELECT
  current_database() AS database_name,
  d.hypertable_schema AS schema_name,
  d.hypertable_name AS hypertable_name,
  d.dimension_number::int AS dimension_number,
  d.column_name AS column_name,
  d.column_type::text AS column_type,
  d.dimension_type::text AS dimension_type,
  d.time_interval::text AS time_interval,
  d.integer_interval::text AS integer_interval,
  d.integer_now_func::text AS integer_now_func,
  COALESCE(
    (to_jsonb(d)->>'num_partitions')::int,
    (to_jsonb(d)->>'num_slices')::int
  ) AS num_slices,
  COALESCE(
    to_jsonb(d)->>'partitioning_func',
    to_jsonb(d)->>'partitioning_func_name'
  ) AS partitioning_func
FROM timescaledb_information.dimensions d
ORDER BY d.hypertable_schema, d.hypertable_name, d.dimension_number
`

const chunksSQL = `
SELECT
  current_database() AS database_name,
  c.hypertable_schema AS schema_name,
  c.hypertable_name AS hypertable_name,
  c.chunk_schema AS chunk_schema,
  c.chunk_name AS chunk_name,
  c.range_start AS range_start,
  c.range_end AS range_end,
  c.range_start_integer AS range_start_integer,
  c.range_end_integer AS range_end_integer,
  COALESCE(c.is_compressed, false) AS is_compressed,
  COALESCE(c.chunk_tablespace, '') AS chunk_tablespace,
  COALESCE(pg_total_relation_size(format('%I.%I', c.chunk_schema, c.chunk_name)::regclass), 0) AS total_size_bytes,
  COALESCE(pg_relation_size(format('%I.%I', c.chunk_schema, c.chunk_name)::regclass), 0) AS data_size_bytes,
  COALESCE(pg_indexes_size(format('%I.%I', c.chunk_schema, c.chunk_name)::regclass), 0) AS index_size_bytes,
  COALESCE((to_jsonb(c)->>'before_compression_total_bytes')::bigint, 0) AS before_compression_bytes,
  COALESCE((to_jsonb(c)->>'after_compression_total_bytes')::bigint, 0) AS after_compression_bytes
FROM timescaledb_information.chunks c
ORDER BY c.hypertable_schema, c.hypertable_name, c.chunk_name
`

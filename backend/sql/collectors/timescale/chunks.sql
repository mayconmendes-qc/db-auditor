-- Chunk inventory facts for the current database (read-only).
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
  COALESCE(pg_indexes_size(format('%I.%I', c.chunk_schema, c.chunk_name)::regclass), 0) AS index_size_bytes
FROM timescaledb_information.chunks c
ORDER BY c.hypertable_schema, c.hypertable_name, c.chunk_name;

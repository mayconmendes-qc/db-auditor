-- timescale.hypertables — is_distributed sunset in TimescaleDB v2.14+.
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
ORDER BY h.hypertable_schema, h.hypertable_name;

-- timescale.dimensions — num_slices renamed to num_partitions on newer views.
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
ORDER BY d.hypertable_schema, d.hypertable_name, d.dimension_number;

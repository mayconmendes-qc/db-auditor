-- Continuous aggregate inventory (read-only).
SELECT
  current_database() AS database_name,
  ca.view_schema AS schema_name,
  ca.view_name AS view_name,
  ca.view_owner AS owner_name,
  ca.materialization_hypertable_schema AS materialization_schema,
  ca.materialization_hypertable_name AS materialization_hypertable,
  COALESCE(ca.materialized_only, false) AS materialized_only,
  COALESCE(ca.compression_enabled, false) AS compression_enabled,
  ca.finalized
FROM timescaledb_information.continuous_aggregates ca
ORDER BY ca.view_schema, ca.view_name;

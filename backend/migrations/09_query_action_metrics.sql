-- Preserve existing measurements while enabling workload-based validation.
ALTER TABLE finding_action_measurement
  DROP CONSTRAINT IF EXISTS finding_action_measurement_metric_check;
ALTER TABLE finding_action_measurement
  ADD CONSTRAINT finding_action_measurement_metric_check
  CHECK (metric IN ('table_size_bytes','finding_observed','query_mean_latency_us','query_reads_per_1000_calls'));

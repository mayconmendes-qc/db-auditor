export type NavigationSection =
  | "Visão geral"
  | "Ambientes"
  | "Audit runs"
  | "Inventário"
  | "Findings";

export interface HealthResponse {
  status: string;
}

export interface Environment {
  id: string;
  name: string;
  type: string;
  discovery_mode: string;
  active: boolean;
  created_at: string;
  updated_at: string;
}

export interface EnvironmentsResponse {
  items: Environment[];
}

export interface DatabaseSnapshot {
  id: string;
  audit_run_id: string;
  environment_id: string;
  database_name: string;
  owner_name: string | null;
  encoding: string | null;
  size_bytes: number;
  connection_count: number;
  allow_connections: boolean;
  is_template: boolean;
  collected_at: string;
}

export interface SchemaSnapshot {
  id: string;
  audit_run_id: string;
  environment_id: string;
  database_name: string;
  schema_name: string;
  owner_name: string | null;
  table_count: number;
  view_count: number;
  materialized_view_count: number;
  sequence_count: number;
  function_count: number;
  size_bytes: number;
  collected_at: string;
}

export interface HypertableSnapshot {
  id: string;
  audit_run_id: string;
  environment_id: string;
  database_name: string;
  schema_name: string;
  hypertable_name: string;
  owner_name: string | null;
  num_dimensions: number;
  num_chunks: number;
  compression_enabled: boolean;
  is_distributed: boolean;
  total_size_bytes: number;
  data_size_bytes: number;
  index_size_bytes: number;
  collected_at: string;
}

export interface DimensionSnapshot {
  id: string;
  database_name: string;
  schema_name: string;
  hypertable_name: string;
  dimension_number: number;
  column_name: string;
  column_type: string | null;
  dimension_type: string | null;
  time_interval: string | null;
  collected_at: string;
}

export interface ChunkSnapshot {
  id: string;
  database_name: string;
  schema_name: string;
  hypertable_name: string;
  chunk_schema: string;
  chunk_name: string;
  range_start: string | null;
  range_end: string | null;
  is_compressed: boolean;
  total_size_bytes: number;
  collected_at: string;
}

export interface CAGGSnapshot {
  id: string;
  database_name: string;
  schema_name: string;
  view_name: string;
  owner_name: string | null;
  materialization_schema: string | null;
  materialization_hypertable: string | null;
  materialized_only: boolean;
  compression_enabled: boolean;
  collected_at: string;
}

export interface JobSnapshot {
  id: string;
  database_name: string;
  job_id: number;
  application_name: string | null;
  proc_name: string | null;
  scheduled: boolean;
  schedule_interval: string | null;
  next_start: string | null;
  hypertable_schema: string | null;
  hypertable_name: string | null;
  collected_at: string;
}

export interface PolicySnapshot {
  id: string;
  database_name: string;
  job_id: number;
  policy_type: string;
  proc_name: string | null;
  hypertable_schema: string | null;
  hypertable_name: string | null;
  schedule_interval: string | null;
  scheduled: boolean;
  next_start: string | null;
  collected_at: string;
}

export interface ItemsResponse<T> {
  items: T[];
}

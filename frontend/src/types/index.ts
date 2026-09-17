export type NavigationSection =
  | "Visão geral"
  | "Ambientes"
  | "Audit runs"
  | "Inventário"
  | "Mappings"
  | "Schema Drift"
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

export interface AuditRun {
  id: string;
  environment_id: string;
  profile: string;
  status: string;
  service_version: string;
  collector_version: string;
  started_at: string;
  finished_at?: string | null;
  warnings: string[];
  errors: string[];
}

export interface CollectorRun {
  id: string;
  audit_run_id: string;
  collector_name: string;
  collector_version: string;
  status: string;
  started_at: string;
  finished_at?: string | null;
  rows_collected: number;
  warning?: string | null;
  error?: string | null;
}

export interface ObjectMapping {
  id: string;
  source_environment_id: string;
  target_environment_id: string;
  source_database: string;
  source_schema: string;
  source_object_type: string;
  source_object_name: string;
  target_database: string;
  target_schema: string;
  target_object_type: string;
  target_object_name: string;
  relation_type: string;
  confidence: number;
  status: string;
  source_fingerprint?: string | null;
  target_fingerprint?: string | null;
  fingerprint_algorithm?: string | null;
  notes?: string | null;
  created_at: string;
  updated_at: string;
}

export interface MappingCandidate {
  source_database: string;
  source_schema: string;
  source_object_type: string;
  source_object_name: string;
  target_database: string;
  target_schema: string;
  target_object_type: string;
  target_object_name: string;
  relation_type: string;
  confidence: number;
  source_fingerprint?: string;
  target_fingerprint?: string;
}

export interface CompareObjectItem {
  object_type: string;
  key: string;
  name: string;
  fingerprint?: string;
  fields?: Record<string, string>;
}

export interface FieldDiff {
  field: string;
  source?: string;
  target?: string;
}

export interface ObjectDiff {
  object_type: string;
  object_key: string;
  status: string;
  source_name?: string;
  target_name?: string;
  source_fingerprint?: string;
  target_fingerprint?: string;
  field_diffs?: FieldDiff[];
}

export interface CompareSummary {
  match: number;
  drift: number;
  only_source: number;
  only_target: number;
  unknown: number;
  total: number;
}

export interface CompareResult {
  source_run_id?: string;
  target_run_id?: string;
  summary: CompareSummary;
  objects: ObjectDiff[];
}

export interface Finding {
  id: string;
  environment_id: string;
  audit_run_id?: string | null;
  finding_type: string;
  severity: string;
  status: string;
  title: string;
  summary: string;
  object_type?: string;
  object_key?: string;
  database_name?: string;
  schema_name?: string;
  object_name?: string;
  evidence?: Record<string, unknown>;
  dedup_key: string;
  first_seen_at: string;
  last_seen_at: string;
  resolved_at?: string | null;
  notes?: string | null;
  created_at: string;
  updated_at: string;
}

export interface AnalyzeResult {
  produced: number;
  saved: number;
  items: Finding[];
}

export interface ItemsResponse<T> {
  items: T[];
}

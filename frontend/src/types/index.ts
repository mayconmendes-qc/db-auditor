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

export interface ItemsResponse<T> {
  items: T[];
}

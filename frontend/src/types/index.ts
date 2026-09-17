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

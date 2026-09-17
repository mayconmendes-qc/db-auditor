/** Shared frontend types. Domain rules stay on the API. */

export type HealthStatus = "ok" | "unavailable" | "unknown";

export interface HealthResponse {
  status: HealthStatus;
}

export type NavigationSection =
  | "Visão geral"
  | "Ambientes"
  | "Audit runs"
  | "Inventário"
  | "Findings";

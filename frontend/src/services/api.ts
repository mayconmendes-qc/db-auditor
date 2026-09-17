import type {
  CAGGSnapshot,
  ChunkSnapshot,
  DatabaseSnapshot,
  DimensionSnapshot,
  EnvironmentsResponse,
  HealthResponse,
  HypertableSnapshot,
  ItemsResponse,
  JobSnapshot,
  PolicySnapshot,
  SchemaSnapshot,
} from "../types";

const API_BASE = import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8080";

async function getJSON<T>(path: string): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`);
  if (!response.ok) {
    throw new Error(`API ${path} failed with ${response.status}`);
  }
  return response.json() as Promise<T>;
}

/** Typed API client — frontend never talks to databases directly. */
export const api = {
  health: () => getJSON<HealthResponse>("/health"),
  ready: () => getJSON<HealthResponse>("/ready"),
  environments: () => getJSON<EnvironmentsResponse>("/api/v1/environments"),
  databases: (environmentId: string) =>
    getJSON<ItemsResponse<DatabaseSnapshot>>(
      `/api/v1/environments/${environmentId}/databases`,
    ),
  schemas: (environmentId: string) =>
    getJSON<ItemsResponse<SchemaSnapshot>>(
      `/api/v1/environments/${environmentId}/schemas`,
    ),
  hypertables: (environmentId: string) =>
    getJSON<ItemsResponse<HypertableSnapshot>>(
      `/api/v1/environments/${environmentId}/hypertables`,
    ),
  dimensions: (environmentId: string) =>
    getJSON<ItemsResponse<DimensionSnapshot>>(
      `/api/v1/environments/${environmentId}/dimensions`,
    ),
  chunks: (environmentId: string) =>
    getJSON<ItemsResponse<ChunkSnapshot>>(
      `/api/v1/environments/${environmentId}/chunks`,
    ),
  continuousAggregates: (environmentId: string) =>
    getJSON<ItemsResponse<CAGGSnapshot>>(
      `/api/v1/environments/${environmentId}/continuous-aggregates`,
    ),
  jobs: (environmentId: string) =>
    getJSON<ItemsResponse<JobSnapshot>>(
      `/api/v1/environments/${environmentId}/jobs`,
    ),
  policies: (environmentId: string) =>
    getJSON<ItemsResponse<PolicySnapshot>>(
      `/api/v1/environments/${environmentId}/policies`,
    ),
};

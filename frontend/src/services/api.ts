import type {
  AuditRun,
  CAGGSnapshot,
  ChunkSnapshot,
  CollectorRun,
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

async function postJSON<T>(path: string, body: unknown): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!response.ok) {
    const text = await response.text();
    throw new Error(text || `API ${path} failed with ${response.status}`);
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
  auditRuns: (params?: {
    environment_id?: string;
    profile?: string;
    status?: string;
  }) => {
    const q = new URLSearchParams();
    if (params?.environment_id) {
      q.set("environment_id", params.environment_id);
    }
    if (params?.profile) {
      q.set("profile", params.profile);
    }
    if (params?.status) {
      q.set("status", params.status);
    }
    const qs = q.toString();
    return getJSON<ItemsResponse<AuditRun>>(
      `/api/v1/audit-runs${qs ? `?${qs}` : ""}`,
    );
  },
  auditRun: (id: string) => getJSON<AuditRun>(`/api/v1/audit-runs/${id}`),
  auditRunCollectors: (id: string) =>
    getJSON<ItemsResponse<CollectorRun>>(`/api/v1/audit-runs/${id}/collectors`),
  triggerAuditRun: (environmentId: string, profile = "manual") =>
    postJSON<{ audit_run_id: string; status: string }>("/api/v1/audit-runs", {
      environment_id: environmentId,
      profile,
    }),
};

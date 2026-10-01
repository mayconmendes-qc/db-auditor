import { networkApiError, toApiError } from "../lib/errors";
import type {
  AnalysisRun,
  AnalyzeResult,
  AuditRun,
  AuditRunCoverage,
  CAGGSnapshot,
  ChunkSnapshot,
  CollectorRun,
  ColumnSnapshot,
  ColumnStatSnapshot,
  CompareObjectItem,
  CompareResult,
  ConnectionStatus,
  DashboardKPIs,
  DatabaseSnapshot,
  DimensionSnapshot,
  EnvironmentsResponse,
  Finding,
  FindingsTrendResponse,
  FunctionSnapshot,
  HealthResponse,
  HypertableSnapshot,
  IndexSnapshot,
  ItemsResponse,
  JobHealthResponse,
  JobSnapshot,
  MappingCandidate,
  ObjectMapping,
  PagedResponse,
  PolicySnapshot,
  SchemaSnapshot,
  ScopeHistoryPoint,
  SnapshotCompleteness,
  StatusResponse,
  StorageGrowthResponse,
  TableHistoryPoint,
  TableSnapshot,
  ViewSnapshot,
  WorkloadSnapshot,
} from "../types";

const API_BASE = import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8080";

async function getJSON<T>(path: string): Promise<T> {
  let response: Response;
  try {
    response = await fetch(`${API_BASE}${path}`);
  } catch (cause) {
    throw networkApiError(path, cause);
  }
  if (!response.ok) {
    throw await toApiError(response, path);
  }
  return response.json() as Promise<T>;
}

async function postJSON<T>(path: string, body: unknown): Promise<T> {
  let response: Response;
  try {
    response = await fetch(`${API_BASE}${path}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
  } catch (cause) {
    throw networkApiError(path, cause);
  }
  if (!response.ok) {
    throw await toApiError(response, path);
  }
  return response.json() as Promise<T>;
}

async function patchJSON<T>(path: string, body: unknown): Promise<T> {
  let response: Response;
  try {
    response = await fetch(`${API_BASE}${path}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
  } catch (cause) {
    throw networkApiError(path, cause);
  }
  if (!response.ok) {
    throw await toApiError(response, path);
  }
  return response.json() as Promise<T>;
}

function qs(params?: Record<string, string | number | undefined>): string {
  if (!params) {
    return "";
  }
  const q = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== "") {
      q.set(k, String(v));
    }
  }
  const s = q.toString();
  return s ? `?${s}` : "";
}

export type InventoryListParams = {
  limit?: number;
  offset?: number;
  q?: string;
  database?: string;
  schema?: string;
  table?: string;
};

export type AnalyticsParams = {
  environment_id?: string;
};

export type TableScopeParams = {
  database: string;
  schema: string;
  table: string;
  from?: string;
  to?: string;
  granularity?: "hour" | "day" | "week" | "month";
};

/** Typed API client — frontend never talks to databases directly. */
export const api = {
  health: () => getJSON<HealthResponse>("/health"),
  ready: () => getJSON<HealthResponse>("/ready"),
  status: () => getJSON<StatusResponse>("/api/v1/status"),
  connectionStatus: () =>
    getJSON<ItemsResponse<ConnectionStatus>>(
      "/api/v1/environments/connection-status",
    ),
  analyticsKpis: (params?: AnalyticsParams) =>
    getJSON<DashboardKPIs>(`/api/v1/analytics/kpis${qs(params)}`),
  analyticsStorage: (params?: AnalyticsParams) =>
    getJSON<StorageGrowthResponse>(`/api/v1/analytics/storage${qs(params)}`),
  analyticsFindingsTrends: (params?: AnalyticsParams) =>
    getJSON<FindingsTrendResponse>(
      `/api/v1/analytics/findings-trends${qs(params)}`,
    ),
  analyticsJobHealth: (params?: AnalyticsParams) =>
    getJSON<JobHealthResponse>(`/api/v1/analytics/job-health${qs(params)}`),
  reportInventory: (params?: AnalyticsParams) =>
    getJSON<Record<string, unknown>>(`/api/v1/reports/inventory${qs(params)}`),
  reportFindings: (params?: AnalyticsParams) =>
    getJSON<Record<string, unknown>>(`/api/v1/reports/findings${qs(params)}`),
  environments: () => getJSON<EnvironmentsResponse>("/api/v1/environments"),
  databases: (environmentId: string) =>
    getJSON<ItemsResponse<DatabaseSnapshot>>(
      `/api/v1/environments/${environmentId}/databases`,
    ),
  schemas: (environmentId: string) =>
    getJSON<ItemsResponse<SchemaSnapshot>>(
      `/api/v1/environments/${environmentId}/schemas`,
    ),
  snapshotStatus: (environmentId: string, auditRunId?: string) =>
    getJSON<SnapshotCompleteness>(
      `/api/v1/environments/${environmentId}/snapshot-status${qs({
        audit_run_id: auditRunId,
      })}`,
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
  tables: (environmentId: string, params?: InventoryListParams) =>
    getJSON<PagedResponse<TableSnapshot>>(
      `/api/v1/environments/${environmentId}/tables${qs(params)}`,
    ),
  tableHistory: (environmentId: string, params: TableScopeParams) =>
    getJSON<ItemsResponse<TableHistoryPoint>>(
      `/api/v1/environments/${environmentId}/tables/history${qs(params)}`,
    ),
  storageHistory: (
    environmentId: string,
    params: Partial<TableScopeParams> & {
      scope: "environment" | "database" | "schema" | "table";
    },
  ) =>
    getJSON<ItemsResponse<ScopeHistoryPoint>>(
      `/api/v1/environments/${environmentId}/history${qs(params)}`,
    ),
  tableColumnStats: (environmentId: string, params: TableScopeParams) =>
    getJSON<ItemsResponse<ColumnStatSnapshot>>(
      `/api/v1/environments/${environmentId}/tables/column-stats${qs(params)}`,
    ),
  tableWorkload: (environmentId: string, params: TableScopeParams) =>
    getJSON<ItemsResponse<WorkloadSnapshot>>(
      `/api/v1/environments/${environmentId}/tables/workload${qs(params)}`,
    ),
  columns: (environmentId: string, params?: InventoryListParams) =>
    getJSON<PagedResponse<ColumnSnapshot>>(
      `/api/v1/environments/${environmentId}/columns${qs(params)}`,
    ),
  indexes: (environmentId: string, params?: InventoryListParams) =>
    getJSON<PagedResponse<IndexSnapshot>>(
      `/api/v1/environments/${environmentId}/indexes${qs(params)}`,
    ),
  views: (environmentId: string, params?: InventoryListParams) =>
    getJSON<PagedResponse<ViewSnapshot>>(
      `/api/v1/environments/${environmentId}/views${qs(params)}`,
    ),
  functions: (environmentId: string, params?: InventoryListParams) =>
    getJSON<PagedResponse<FunctionSnapshot>>(
      `/api/v1/environments/${environmentId}/functions${qs(params)}`,
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
    const s = q.toString();
    return getJSON<ItemsResponse<AuditRun>>(
      `/api/v1/audit-runs${s ? `?${s}` : ""}`,
    );
  },
  auditRun: (id: string) => getJSON<AuditRun>(`/api/v1/audit-runs/${id}`),
  auditRunCollectors: (id: string) =>
    getJSON<ItemsResponse<CollectorRun>>(`/api/v1/audit-runs/${id}/collectors`),
  auditRunCoverage: (id: string) =>
    getJSON<ItemsResponse<AuditRunCoverage>>(
      `/api/v1/audit-runs/${id}/coverage`,
    ),
  auditRunAnalysis: (id: string) =>
    getJSON<AnalysisRun>(`/api/v1/audit-runs/${id}/analysis`),
  reprocessAuditRun: (id: string) =>
    postJSON<{ audit_run_id: string; produced: number; saved: number }>(
      `/api/v1/audit-runs/${id}/reprocess`,
      {},
    ),
  triggerAuditRun: (environmentId: string, profile = "manual") =>
    postJSON<{ audit_run_id: string; status: string }>("/api/v1/audit-runs", {
      environment_id: environmentId,
      profile,
    }),
  mappings: (params?: {
    source_environment_id?: string;
    target_environment_id?: string;
    status?: string;
  }) => {
    const q = new URLSearchParams();
    if (params?.source_environment_id) {
      q.set("source_environment_id", params.source_environment_id);
    }
    if (params?.target_environment_id) {
      q.set("target_environment_id", params.target_environment_id);
    }
    if (params?.status) {
      q.set("status", params.status);
    }
    const s = q.toString();
    return getJSON<ItemsResponse<ObjectMapping>>(
      `/api/v1/mappings${s ? `?${s}` : ""}`,
    );
  },
  createMapping: (body: Partial<ObjectMapping>) =>
    postJSON<ObjectMapping>("/api/v1/mappings", body),
  updateMappingStatus: (id: string, status: string, notes?: string) =>
    patchJSON<ObjectMapping>(`/api/v1/mappings/${id}`, { status, notes }),
  suggestMappings: (body: {
    source: Array<Record<string, string>>;
    target: Array<Record<string, string>>;
    target_default_db?: string;
  }) =>
    postJSON<ItemsResponse<MappingCandidate>>("/api/v1/mappings/suggest", body),
  compare: (body: {
    source_run_id?: string;
    target_run_id?: string;
    source: CompareObjectItem[];
    target: CompareObjectItem[];
    statuses?: string[];
    object_type?: string;
  }) => postJSON<CompareResult>("/api/v1/compare", body),
  findings: (params?: {
    environment_id?: string;
    finding_type?: string;
    severity?: string;
    status?: string;
    limit?: number;
  }) => {
    const q = new URLSearchParams();
    if (params?.environment_id) {
      q.set("environment_id", params.environment_id);
    }
    if (params?.finding_type) {
      q.set("finding_type", params.finding_type);
    }
    if (params?.severity) {
      q.set("severity", params.severity);
    }
    if (params?.status) {
      q.set("status", params.status);
    }
    if (params?.limit) {
      q.set("limit", String(params.limit));
    }
    const s = q.toString();
    return getJSON<ItemsResponse<Finding>>(
      `/api/v1/findings${s ? `?${s}` : ""}`,
    );
  },
  finding: (id: string) => getJSON<Finding>(`/api/v1/findings/${id}`),
  updateFindingStatus: (id: string, status: string, notes?: string) =>
    patchJSON<Finding>(`/api/v1/findings/${id}`, { status, notes }),
  analyzeFindings: (body: {
    environment_id: string;
    audit_run_id?: string;
    tables?: unknown[];
    indexes?: unknown[];
    hypertables?: unknown[];
    chunks?: unknown[];
    caggs?: unknown[];
    policies?: unknown[];
    jobs?: unknown[];
    activity?: unknown[];
    vacuum?: unknown[];
    locks?: unknown[];
    connections?: unknown[];
    query_stats?: unknown[];
    roles?: unknown[];
    grants?: unknown[];
    functions?: unknown[];
  }) => postJSON<AnalyzeResult>("/api/v1/findings/analyze", body),
};

/** Generated from backend/api/openapi.json. Do not edit. */
export const apiContractVersion = "0.1.0";

export type ApiPath =
  | "/api/v1/analytics/findings-trends"
  | "/api/v1/analytics/job-health"
  | "/api/v1/analytics/kpis"
  | "/api/v1/analytics/storage"
  | "/api/v1/audit-runs"
  | "/api/v1/audit-runs/{id}"
  | "/api/v1/audit-runs/{id}/analysis"
  | "/api/v1/audit-runs/{id}/cancel"
  | "/api/v1/audit-runs/{id}/collectors"
  | "/api/v1/audit-runs/{id}/coverage"
  | "/api/v1/audit-runs/{id}/databases/{database}/cancel"
  | "/api/v1/audit-runs/{id}/reprocess"
  | "/api/v1/auth/login"
  | "/api/v1/auth/logout"
  | "/api/v1/auth/me"
  | "/api/v1/auth/users"
  | "/api/v1/compare"
  | "/api/v1/environments"
  | "/api/v1/environments/connection-status"
  | "/api/v1/environments/{id}/baseline"
  | "/api/v1/environments/{id}/baseline/comparisons"
  | "/api/v1/environments/{id}/chunks"
  | "/api/v1/environments/{id}/columns"
  | "/api/v1/environments/{id}/constraints"
  | "/api/v1/environments/{id}/continuous-aggregates"
  | "/api/v1/environments/{id}/databases"
  | "/api/v1/environments/{id}/dimensions"
  | "/api/v1/environments/{id}/functions"
  | "/api/v1/environments/{id}/history"
  | "/api/v1/environments/{id}/hypertables"
  | "/api/v1/environments/{id}/indexes"
  | "/api/v1/environments/{id}/jobs"
  | "/api/v1/environments/{id}/policies"
  | "/api/v1/environments/{id}/reports"
  | "/api/v1/environments/{id}/reports/{job}"
  | "/api/v1/environments/{id}/reports/{job}/cancel"
  | "/api/v1/environments/{id}/reports/{job}/download"
  | "/api/v1/environments/{id}/reports/{job}/retry"
  | "/api/v1/environments/{id}/rules"
  | "/api/v1/environments/{id}/rules/{rule}"
  | "/api/v1/environments/{id}/runs/{run}/databases/{database}/schemas/{schema}/tables/{table}/assessment"
  | "/api/v1/environments/{id}/runs/{run}/databases/{database}/schemas/{schema}/tables/{table}/dependencies"
  | "/api/v1/environments/{id}/runs/{run}/databases/{database}/schemas/{schema}/tables/{table}/findings"
  | "/api/v1/environments/{id}/runs/{run}/databases/{database}/schemas/{schema}/tables/{table}/grants"
  | "/api/v1/environments/{id}/runs/{run}/databases/{database}/schemas/{schema}/tables/{table}/graph"
  | "/api/v1/environments/{id}/runs/{run}/databases/{database}/schemas/{schema}/tables/{table}/rls-policies"
  | "/api/v1/environments/{id}/runs/{run}/databases/{database}/schemas/{schema}/tables/{table}/triggers"
  | "/api/v1/environments/{id}/runs/{run}/score"
  | "/api/v1/environments/{id}/runs/{run}/scores"
  | "/api/v1/environments/{id}/schedules"
  | "/api/v1/environments/{id}/schemas"
  | "/api/v1/environments/{id}/snapshot-status"
  | "/api/v1/environments/{id}/tables"
  | "/api/v1/environments/{id}/tables/column-stats"
  | "/api/v1/environments/{id}/tables/history"
  | "/api/v1/environments/{id}/tables/workload"
  | "/api/v1/environments/{id}/views"
  | "/api/v1/finding-categories/{category}"
  | "/api/v1/findings"
  | "/api/v1/findings/analyze"
  | "/api/v1/findings/{id}"
  | "/api/v1/findings/{id}/timeline"
  | "/api/v1/mappings"
  | "/api/v1/mappings/suggest"
  | "/api/v1/mappings/{id}"
  | "/api/v1/reports/findings"
  | "/api/v1/reports/inventory"
  | "/api/v1/server-compare"
  | "/api/v1/status";

export interface ScopeScore {
  version: string;
  environment_id: string;
  audit_run_id: string;
  database_name?: string;
  schema_name?: string;
  table_name?: string;
  status: string;
  score: number | null;
  confidence: number;
  categories: ScopeScoreCategory[];
  missing_collectors: string[];
}

export interface ScopeScoreCategory {
  category: string;
  score: number;
  penalty: number;
  findings: number;
}

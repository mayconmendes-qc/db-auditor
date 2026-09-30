package api

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/mayconmendes-qc/db-auditor/internal/repository"
)

// DashboardKPIs is the executive summary for the main dashboard.
type DashboardKPIs struct {
	GeneratedAtUTC       time.Time `json:"generated_at_utc"`
	Environments         int       `json:"environments"`
	OpenFindings         int       `json:"open_findings"`
	CriticalFindings     int       `json:"critical_findings"`
	HighFindings         int       `json:"high_findings"`
	FailedRunsRecent     int       `json:"failed_runs_recent"`
	SuccessfulRunsRecent int       `json:"successful_runs_recent"`
	LastRunAt            *string   `json:"last_run_at,omitempty"`
	LastRunStatus        *string   `json:"last_run_status,omitempty"`
}

func registerAnalyticsRoutes(mux *http.ServeMux, store AnalyticsStore) {
	mux.HandleFunc("GET /api/v1/analytics/kpis", getKPIs(store))
	mux.HandleFunc("GET /api/v1/analytics/storage", getStorage(store))
	mux.HandleFunc("GET /api/v1/analytics/findings-trends", getFindingsTrends(store))
	mux.HandleFunc("GET /api/v1/analytics/job-health", getJobHealth(store))
	mux.HandleFunc("GET /api/v1/analytics/reports/findings", getFindingsReport(store))
}

func getKPIs(store AnalyticsStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		envs, err := store.ListEnvironments(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Não foi possível listar os ambientes.")
			return
		}
		findings, err := store.ListFindings(r.Context(), repository.FindingFilter{Status: "open", Limit: 5000})
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Não foi possível listar os findings.")
			return
		}
		runs, err := store.ListAuditRuns(r.Context(), repository.AuditRunFilter{Limit: 50})
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Não foi possível listar as execuções.")
			return
		}

		critical, high := 0, 0
		for _, f := range findings {
			switch strings.ToLower(f.Severity) {
			case "critical":
				critical++
			case "high":
				high++
			}
		}

		failed, success := 0, 0
		var lastAt *string
		var lastStatus *string
		for i, run := range runs {
			if i == 0 {
				s := run.StartedAt.UTC().Format(time.RFC3339)
				lastAt = &s
				st := run.Status
				lastStatus = &st
			}
			if isFailedRunStatus(run.Status) {
				failed++
			} else if isSuccessfulRunStatus(run.Status) {
				success++
			}
		}

		writeJSON(w, http.StatusOK, DashboardKPIs{
			GeneratedAtUTC:       time.Now().UTC(),
			Environments:         len(envs),
			OpenFindings:         len(findings),
			CriticalFindings:     critical,
			HighFindings:         high,
			FailedRunsRecent:     failed,
			SuccessfulRunsRecent: success,
			LastRunAt:            lastAt,
			LastRunStatus:        lastStatus,
		})
	}
}

func getStorage(store AnalyticsStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := store.StorageByEnvironment(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Não foi possível obter o armazenamento.")
			return
		}
		if items == nil {
			items = []repository.StorageByEnv{}
		}
		writeJSON(w, http.StatusOK, items)
	}
}

func getFindingsTrends(store AnalyticsStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		days := 14
		if v := r.URL.Query().Get("days"); v != "" {
			if n, err := parsePositiveInt(v, 1, 90); err == nil {
				days = n
			}
		}
		items, err := store.FindingsTrends(r.Context(), days)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Não foi possível obter as tendências.")
			return
		}
		if items == nil {
			items = []repository.FindingTrendPoint{}
		}
		writeJSON(w, http.StatusOK, items)
	}
}

func getJobHealth(store AnalyticsStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := store.JobHealth(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Não foi possível obter a saúde dos jobs.")
			return
		}
		if items == nil {
			items = []repository.JobHealthRow{}
		}
		writeJSON(w, http.StatusOK, items)
	}
}

type findingsReport struct {
	GeneratedAtUTC time.Time                    `json:"generated_at_utc"`
	Total          int                          `json:"total"`
	BySeverity     map[string]int               `json:"by_severity"`
	ByStatus       map[string]int               `json:"by_status"`
	Items          []repository.FindingListItem `json:"items"`
}

func getFindingsReport(store AnalyticsStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filter := repository.FindingFilter{
			EnvironmentID: r.URL.Query().Get("environment_id"),
			Severity:      r.URL.Query().Get("severity"),
			Status:        r.URL.Query().Get("status"),
			Analyzer:      r.URL.Query().Get("analyzer"),
			Limit:         2000,
		}
		items, err := store.ListFindings(r.Context(), filter)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Não foi possível gerar o relatório.")
			return
		}
		if items == nil {
			items = []repository.FindingListItem{}
		}
		sev := map[string]int{}
		st := map[string]int{}
		for _, it := range items {
			sev[strings.ToLower(it.Severity)]++
			st[strings.ToLower(it.Status)]++
		}
		writeJSON(w, http.StatusOK, findingsReport{
			GeneratedAtUTC: time.Now().UTC(),
			Total:          len(items),
			BySeverity:     sev,
			ByStatus:       st,
			Items:          items,
		})
	}
}

func bucketsFromMap(m map[string]int) []map[string]any {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		out = append(out, map[string]any{"key": k, "count": m[k]})
	}
	return out
}

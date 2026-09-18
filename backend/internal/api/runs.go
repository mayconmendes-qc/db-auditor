package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/mayconmendes-qc/timescale-auditor/internal/audit"
	"github.com/mayconmendes-qc/timescale-auditor/internal/repository"
	"github.com/mayconmendes-qc/timescale-auditor/internal/scheduler"
)

// RunService exposes audit run operations to HTTP handlers.
type RunService interface {
	ListAuditRuns(ctx context.Context, environmentID, profile, status string, limit int) ([]repository.AuditRunRow, error)
	GetAuditRun(ctx context.Context, id string) (*repository.AuditRunRow, error)
	ListCollectorRuns(ctx context.Context, auditRunID string) ([]repository.CollectorRunRow, error)
}

// ManualRunner triggers an audit run with overlap protection.
type ManualRunner interface {
	TryRun(ctx context.Context, environmentID, profile string) (audit.RunResult, error)
}

type triggerBody struct {
	EnvironmentID string `json:"environment_id"`
	Profile       string `json:"profile"`
}

func registerRunRoutes(mux *http.ServeMux, runs RunService, runner ManualRunner) {
	mux.HandleFunc("GET /api/v1/audit-runs", listAuditRuns(runs))
	mux.HandleFunc("GET /api/v1/audit-runs/{id}", getAuditRun(runs))
	mux.HandleFunc("GET /api/v1/audit-runs/{id}/collectors", listRunCollectors(runs))
	mux.HandleFunc("POST /api/v1/audit-runs", triggerAuditRun(runner))
}

func listAuditRuns(runs RunService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		limit, _ := strconv.Atoi(q.Get("limit"))
		items, err := runs.ListAuditRuns(r.Context(), q.Get("environment_id"), q.Get("profile"), q.Get("status"), limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Não foi possível listar as execuções de auditoria.")
			return
		}
		if items == nil {
			items = []repository.AuditRunRow{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func getAuditRun(runs RunService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		item, err := runs.GetAuditRun(r.Context(), id)
		if err != nil || item == nil {
			writeError(w, http.StatusNotFound, CodeNotFound, "Execução de auditoria não encontrada.")
			return
		}
		writeJSON(w, http.StatusOK, item)
	}
}

func listRunCollectors(runs RunService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		items, err := runs.ListCollectorRuns(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Não foi possível listar os collectors.")
			return
		}
		if items == nil {
			items = []repository.CollectorRunRow{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func triggerAuditRun(runner ManualRunner) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if runner == nil {
			writeError(w, http.StatusServiceUnavailable, CodeUnavailable, "Runner de auditoria indisponível.")
			return
		}
		var body triggerBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, CodeBadRequest, "Corpo da requisição inválido.")
			return
		}
		if body.EnvironmentID == "" {
			writeError(w, http.StatusBadRequest, CodeEnvironmentRequired, "O campo environment_id é obrigatório.")
			return
		}
		if body.Profile == "" {
			body.Profile = audit.ProfileManual
		}
		res, err := runner.TryRun(r.Context(), body.EnvironmentID, body.Profile)
		if err != nil {
			writeError(w, http.StatusConflict, CodeConflict, err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, res)
	}
}

// Ensure scheduler import used when wiring schedules later.
var _ = scheduler.ProfileInterval

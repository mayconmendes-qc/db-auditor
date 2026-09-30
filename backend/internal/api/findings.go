package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/mayconmendes-qc/db-auditor/internal/repository"
)

// FindingStore exposes finding persistence.
type FindingStore interface {
	ListFindings(ctx context.Context, environmentID, findingType, severity, status string, limit int) ([]repository.Finding, error)
	GetFinding(ctx context.Context, id string) (*repository.Finding, error)
	UpdateFindingStatus(ctx context.Context, id, status, notes string) (*repository.Finding, error)
	UpsertFinding(ctx context.Context, p repository.UpsertFindingParams) (*repository.Finding, error)
}

type updateFindingBody struct {
	Status string `json:"status"`
	Notes  string `json:"notes"`
}

type analyzeBody struct {
	EnvironmentID string `json:"environment_id"`
	AuditRunID    string `json:"audit_run_id"`
}

type AnalysisRunner interface {
	AnalyzeRun(ctx context.Context, environmentID, auditRunID string) (produced, saved int, err error)
}

func registerFindingRoutes(mux *http.ServeMux, store FindingStore, analysis AnalysisRunner) {
	mux.HandleFunc("GET /api/v1/findings", listFindings(store))
	mux.HandleFunc("GET /api/v1/findings/{id}", getFinding(store))
	mux.HandleFunc("PATCH /api/v1/findings/{id}", patchFinding(store))
	mux.HandleFunc("POST /api/v1/findings/analyze", analyzeFindings(analysis))
}

func listFindings(store FindingStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		limit, _ := strconv.Atoi(q.Get("limit"))
		items, err := store.ListFindings(r.Context(), q.Get("environment_id"), q.Get("finding_type"), q.Get("severity"), q.Get("status"), limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Não foi possível listar os findings.")
			return
		}
		if items == nil {
			items = []repository.Finding{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func getFinding(store FindingStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		f, err := store.GetFinding(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, CodeNotFound, "Finding não encontrado.")
			return
		}
		writeJSON(w, http.StatusOK, f)
	}
}

func patchFinding(store FindingStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var body updateFindingBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, CodeBadRequest, "Corpo da requisição inválido.")
			return
		}
		if body.Status == "" {
			writeError(w, http.StatusBadRequest, CodeValidation, "O campo status é obrigatório.")
			return
		}
		switch body.Status {
		case "open", "acknowledged", "resolved", "suppressed":
		default:
			writeError(w, http.StatusBadRequest, CodeValidation, "Status inválido. Use open, acknowledged, resolved ou suppressed.")
			return
		}
		f, err := store.UpdateFindingStatus(r.Context(), id, body.Status, body.Notes)
		if err != nil {
			writeError(w, http.StatusNotFound, CodeNotFound, "Finding não encontrado.")
			return
		}
		writeJSON(w, http.StatusOK, f)
	}
}

func analyzeFindings(runner AnalysisRunner) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body analyzeBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, CodeBadRequest, "Corpo da requisição inválido.")
			return
		}
		if body.EnvironmentID == "" || body.AuditRunID == "" {
			writeError(w, http.StatusBadRequest, CodeEnvironmentRequired, "Os campos environment_id e audit_run_id são obrigatórios.")
			return
		}
		if runner == nil {
			writeError(w, http.StatusServiceUnavailable, CodeInternal, "Análise automática indisponível.")
			return
		}
		produced, saved, err := runner.AnalyzeRun(r.Context(), body.EnvironmentID, body.AuditRunID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao executar analyzers.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"produced": produced,
			"saved":    saved,
		})
	}
}

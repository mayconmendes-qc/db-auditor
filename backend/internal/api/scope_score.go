package api

import (
	"context"
	"net/http"

	"github.com/osmendes/db-auditor/internal/repository"
)

type scopeScoreStore interface {
	GetScopeScore(context.Context, string, string, string, string, string) (*repository.ScopeScore, error)
	ListScopeAggregates(context.Context, string, string) ([]repository.ScopeAggregate, error)
}

func registerScopeScoreRoutes(mux *http.ServeMux, store InventoryStore) {
	mux.HandleFunc("GET /api/v1/environments/{id}/runs/{run}/score", func(w http.ResponseWriter, r *http.Request) {
		backend, ok := store.(scopeScoreStore)
		if !ok {
			writeError(w, http.StatusServiceUnavailable, CodeUnavailable, "Score indisponível.")
			return
		}
		env, run := r.PathValue("id"), r.PathValue("run")
		if !uuidPattern.MatchString(env) || !uuidPattern.MatchString(run) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Escopo inválido.")
			return
		}
		q := r.URL.Query()
		database, schema, table := q.Get("database"), q.Get("schema"), q.Get("table")
		if (schema != "" && database == "") || (table != "" && schema == "") {
			writeError(w, http.StatusBadRequest, CodeValidation, "Escopo inválido.")
			return
		}
		item, err := backend.GetScopeScore(r.Context(), env, run, database, schema, table)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao calcular score.")
			return
		}
		if item == nil {
			writeError(w, http.StatusNotFound, CodeNotFound, "Execução não encontrada.")
			return
		}
		writeJSON(w, http.StatusOK, item)
	})

	mux.HandleFunc("GET /api/v1/environments/{id}/runs/{run}/scores", func(w http.ResponseWriter, r *http.Request) {
		backend, ok := store.(scopeScoreStore)
		if !ok {
			writeError(w, http.StatusServiceUnavailable, CodeUnavailable, "Score indisponível.")
			return
		}
		env, run := r.PathValue("id"), r.PathValue("run")
		if !uuidPattern.MatchString(env) || !uuidPattern.MatchString(run) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Escopo inválido.")
			return
		}
		items, err := backend.ListScopeAggregates(r.Context(), env, run)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao agregar scores.")
			return
		}
		if items == nil {
			items = []repository.ScopeAggregate{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "version": "scope-v2"})
	})
	mux.HandleFunc("GET /api/v1/environments/{id}/runs/{run}/score/compare", func(w http.ResponseWriter, r *http.Request) {
		backend, ok := store.(interface {
			GetScopeScore(context.Context, string, string, string, string, string) (*repository.ScopeScore, error)
			GetAuditBaseline(context.Context, string, string, string, string) (*repository.AuditBaseline, error)
		})
		if !ok {
			writeError(w, http.StatusServiceUnavailable, CodeUnavailable, "Comparação indisponível.")
			return
		}
		env, run := r.PathValue("id"), r.PathValue("run")
		q := r.URL.Query()
		database, schema, table := q.Get("database"), q.Get("schema"), q.Get("table")
		if !uuidPattern.MatchString(env) || !uuidPattern.MatchString(run) || !validBaselineScope(database, schema, table) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Escopo inválido.")
			return
		}
		baseline, err := backend.GetAuditBaseline(r.Context(), env, database, schema, table)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao carregar referência.")
			return
		}
		if baseline == nil {
			writeJSON(w, http.StatusOK, map[string]any{"status": "no_baseline", "reason": "Selecione uma execução de referência aprovada para este escopo."})
			return
		}
		current, err := backend.GetScopeScore(r.Context(), env, run, database, schema, table)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao calcular nota atual.")
			return
		}
		previous, err := backend.GetScopeScore(r.Context(), env, baseline.AuditRunID, database, schema, table)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao calcular nota de referência.")
			return
		}
		if current == nil || previous == nil || current.Score == nil || previous.Score == nil || current.Version != previous.Version || current.Profile != previous.Profile || current.CollectorVersion != previous.CollectorVersion || current.RuleVersion != previous.RuleVersion {
			writeJSON(w, http.StatusOK, map[string]any{"status": "incompatible", "reason": "Cobertura, análise ou versão insuficiente para uma comparação confiável.", "current": current, "baseline": previous})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "comparable", "current": current, "baseline": previous, "delta": *current.Score - *previous.Score,
			"reason": "Variação da nota entre execuções com a mesma fórmula e cobertura completa; consulte os achados para explicar as categorias alteradas."})
	})
}

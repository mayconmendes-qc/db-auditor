package api

import (
	"context"
	"net/http"

	"github.com/mayconmendes-qc/db-auditor/internal/repository"
)

type scopeScoreStore interface {
	GetScopeScore(context.Context, string, string, string, string, string) (*repository.ScopeScore, error)
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
}

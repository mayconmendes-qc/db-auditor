package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/mayconmendes-qc/db-auditor/internal/analyzer"
)

type ruleReader interface {
	EffectiveRules(context.Context, string, string) ([]analyzer.EffectiveRule, error)
}

func registerRuleRoutes(mux *http.ServeMux, store InventoryStore) {
	mux.HandleFunc("GET /api/v1/environments/{id}/rules", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeError(w, http.StatusBadRequest, CodeValidation, "Environment obrigatório.")
			return
		}
		reader, ok := store.(ruleReader)
		if !ok {
			writeError(w, http.StatusNotImplemented, CodeInternal, "Catálogo de regras indisponível.")
			return
		}
		schema := strings.TrimSpace(r.URL.Query().Get("schema"))
		items, err := reader.EffectiveRules(r.Context(), id, schema)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Não foi possível obter regras.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "environment_id": id, "schema": schema})
	})
}

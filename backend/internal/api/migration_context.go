package api

import (
	"context"
	"net/http"
	"time"

	"github.com/osmendes/db-auditor/internal/migrationrisk"
	"github.com/osmendes/db-auditor/internal/repository"
)

type migrationContextStore interface {
	GetMigrationContext(context.Context, string, []string, time.Time) (*repository.MigrationContextResponse, error)
}

func registerMigrationContextRoutes(mux *http.ServeMux, store InventoryStore) {
	backend, ok := store.(migrationContextStore)
	if !ok {
		return
	}
	mux.HandleFunc("GET /api/v1/environments/{id}/migration-context", func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		if !uuidPattern.MatchString(env) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente inválido.")
			return
		}
		tables := migrationrisk.ParseTablesCSV(r.URL.Query().Get("tables"))
		if err := migrationrisk.ValidateTablesBound(tables); err != nil {
			writeError(w, http.StatusBadRequest, CodeValidation, err.Error())
			return
		}
		out, err := backend.GetMigrationContext(r.Context(), env, tables, time.Now().UTC())
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao carregar o contexto de migration.")
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
}

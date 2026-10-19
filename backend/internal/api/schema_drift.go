package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/osmendes/db-auditor/internal/repository"
	"github.com/osmendes/db-auditor/internal/schemadiff"
)

type schemaDiffStore interface {
	BuildSchemaDiff(context.Context, repository.SchemaDiffRequest) (*repository.SchemaDiffResult, error)
}

func registerSchemaDriftRoutes(mux *http.ServeMux, store InventoryStore) {
	base := "/api/v1/environments/{id}"
	mux.HandleFunc("GET "+base+"/schema-diff", func(w http.ResponseWriter, r *http.Request) {
		backend, ok := store.(schemaDiffStore)
		if !ok {
			writeError(w, http.StatusServiceUnavailable, CodeUnavailable, "Diff de schema indisponível.")
			return
		}
		env := r.PathValue("id")
		if !uuidPattern.MatchString(env) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente inválido.")
			return
		}
		q := r.URL.Query()
		targetEnv := strings.TrimSpace(q.Get("target_environment_id"))
		if targetEnv == "" {
			targetEnv = env
		}
		if !uuidPattern.MatchString(targetEnv) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente de destino inválido.")
			return
		}
		sourceRun := strings.TrimSpace(q.Get("source_run"))
		targetRun := strings.TrimSpace(q.Get("target_run"))
		if sourceRun != "" && !uuidPattern.MatchString(sourceRun) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Execução de origem inválida.")
			return
		}
		if targetRun != "" && !uuidPattern.MatchString(targetRun) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Execução de destino inválida.")
			return
		}
		limit, offset, valid := parseListPage(r, 50)
		if !valid {
			writeError(w, http.StatusBadRequest, CodeValidation, "Paginação inválida.")
			return
		}
		res, err := backend.BuildSchemaDiff(r.Context(), repository.SchemaDiffRequest{
			SourceEnv:  env,
			SourceRun:  sourceRun,
			TargetEnv:  targetEnv,
			TargetRun:  targetRun,
			ObjectType: strings.TrimSpace(q.Get("object_type")),
			Status:     strings.TrimSpace(q.Get("status")),
			Limit:      limit,
			Offset:     offset,
		})
		if errors.Is(err, schemadiff.ErrMechanismMismatch) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Mecanismos de banco diferentes não podem ser comparados.")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao calcular o diff de schema.")
			return
		}
		writeJSON(w, http.StatusOK, res)
	})
}

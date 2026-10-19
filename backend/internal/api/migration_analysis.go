package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/osmendes/db-auditor/internal/observability"
	"github.com/osmendes/db-auditor/internal/repository"
)

type migrationAnalysisStore interface {
	CreateMigrationAnalysis(context.Context, repository.CreateMigrationAnalysisParams) (*repository.MigrationAnalysis, error)
	ListMigrationAnalyses(context.Context, string, int) ([]repository.MigrationAnalysis, error)
	GetMigrationAnalysis(context.Context, string, string) (*repository.MigrationAnalysis, []repository.MigrationAnalysisItem, error)
}

func registerMigrationAnalysisRoutes(mux *http.ServeMux, store InventoryStore) {
	backend, ok := store.(migrationAnalysisStore)
	if !ok {
		return
	}
	base := "/api/v1/environments/{id}/migration-analyses"
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		if !uuidPattern.MatchString(env) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente inválido.")
			return
		}
		var body struct {
			Repository      string                             `json:"repository"`
			PR              string                             `json:"pr"`
			Commit          string                             `json:"commit"`
			HighestSeverity string                             `json:"highest_severity"`
			RulesVersion    string                             `json:"rules_version"`
			PGVersion       int                                `json:"pg_version"`
			ContextMode     string                             `json:"context_mode"`
			AuditRunID      string                             `json:"audit_run_id"`
			Items           []repository.MigrationAnalysisItem `json:"items"`
		}
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxAPIBody))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&body); err != nil {
			if strings.Contains(err.Error(), "http: request body too large") {
				writeError(w, http.StatusRequestEntityTooLarge, CodeValidation, "O corpo da requisição passa de 1 MiB.")
				return
			}
			writeError(w, http.StatusBadRequest, CodeValidation, "Corpo inválido.")
			return
		}
		tokenID := ""
		if tok := requestEnvToken(r); tok != nil {
			tokenID = tok.ID
		}
		created, err := backend.CreateMigrationAnalysis(r.Context(), repository.CreateMigrationAnalysisParams{
			EnvironmentID: env, Repository: body.Repository, PR: body.PR, CommitSHA: body.Commit,
			HighestSeverity: body.HighestSeverity, RulesVersion: body.RulesVersion,
			PGVersion: body.PGVersion, ContextMode: body.ContextMode, AuditRunID: body.AuditRunID,
			TokenID: tokenID, Items: body.Items,
		})
		if err != nil {
			observability.DefaultMetrics.IncMigrationAnalysis("rejected")
			writeError(w, http.StatusBadRequest, CodeValidation, "Não foi possível registrar a análise.")
			return
		}
		for _, it := range body.Items {
			if it.Code == "MR-PARSE" {
				observability.DefaultMetrics.IncMigrationParseError()
			}
		}
		observability.DefaultMetrics.IncMigrationAnalysis(created.HighestSeverity)
		writeJSON(w, http.StatusCreated, created)
	})
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		if !uuidPattern.MatchString(env) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente inválido.")
			return
		}
		items, err := backend.ListMigrationAnalyses(r.Context(), env, 50)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao listar análises.")
			return
		}
		if items == nil {
			items = []repository.MigrationAnalysis{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	})
	mux.HandleFunc("GET "+base+"/{analysis}", func(w http.ResponseWriter, r *http.Request) {
		env, id := r.PathValue("id"), r.PathValue("analysis")
		if !uuidPattern.MatchString(env) || !uuidPattern.MatchString(id) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Identificador inválido.")
			return
		}
		header, items, err := backend.GetMigrationAnalysis(r.Context(), env, id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao carregar a análise.")
			return
		}
		if header == nil {
			writeError(w, http.StatusNotFound, CodeNotFound, "Análise não encontrada.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"analysis": header, "items": items})
	})
}

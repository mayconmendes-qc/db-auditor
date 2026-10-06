package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/osmendes/db-auditor/internal/repository"
)

func registerActionMeasurementRoutes(mux *http.ServeMux, store InventoryStore) {
	backend, ok := store.(interface {
		ListTrackedActions(context.Context, string) ([]repository.TrackedAction, error)
		ListActionMeasurements(context.Context, string) ([]repository.ActionMeasurement, error)
		RecordActionMeasurement(context.Context, string, string, string, string, string, string, string) (*repository.ActionMeasurement, error)
	})
	if !ok {
		return
	}
	mux.HandleFunc("GET /api/v1/environments/{id}/actions", func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		if !uuidPattern.MatchString(env) {
			writeError(w, 400, CodeValidation, "Ambiente inválido.")
			return
		}
		items, err := backend.ListTrackedActions(r.Context(), env)
		if err != nil {
			writeError(w, 500, CodeInternal, "Falha ao listar ações.")
			return
		}
		writeJSON(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("GET /api/v1/findings/{id}/action/measurements", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !uuidPattern.MatchString(id) {
			writeError(w, 400, CodeValidation, "Achado inválido.")
			return
		}
		items, err := backend.ListActionMeasurements(r.Context(), id)
		if err != nil {
			writeError(w, 500, CodeInternal, "Falha ao listar medições.")
			return
		}
		writeJSON(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("POST /api/v1/findings/{id}/action/measurements", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var body struct {
			Before     string `json:"before_run_id"`
			After      string `json:"after_run_id"`
			Metric     string `json:"metric"`
			Hypothesis string `json:"hypothesis"`
			Window     string `json:"window_note"`
		}
		if !uuidPattern.MatchString(id) || json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body) != nil || !uuidPattern.MatchString(body.Before) || !uuidPattern.MatchString(body.After) {
			writeError(w, 400, CodeValidation, "Execuções inválidas.")
			return
		}
		body.Hypothesis, body.Window = strings.TrimSpace(body.Hypothesis), strings.TrimSpace(body.Window)
		if (body.Metric != "table_size_bytes" && body.Metric != "finding_observed") || len(body.Hypothesis) < 8 || len(body.Hypothesis) > 1000 || len(body.Window) < 4 || len(body.Window) > 500 {
			writeError(w, 400, CodeValidation, "Informe métrica, hipótese e janela de observação.")
			return
		}
		actor := "local"
		if user := requestIdentity(r); user != nil {
			actor = user.Username
		}
		item, err := backend.RecordActionMeasurement(r.Context(), id, body.Before, body.After, body.Metric, body.Hypothesis, body.Window, actor)
		if errors.Is(err, repository.ErrMeasurementRun) || errors.Is(err, repository.ErrActionNotFound) {
			writeError(w, 422, CodeValidation, "Execuções ou achado incompatíveis.")
			return
		}
		if err != nil {
			writeError(w, 500, CodeInternal, "Falha ao registrar medição.")
			return
		}
		writeJSON(w, 201, item)
	})
}

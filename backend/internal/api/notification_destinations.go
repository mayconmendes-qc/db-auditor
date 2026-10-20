package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/osmendes/db-auditor/internal/repository"
)

type notificationDestinationStore interface {
	ListNotificationDestinations(context.Context, string) ([]repository.NotificationDestination, error)
	ReplaceNotificationDestinations(context.Context, string, string, []repository.NotificationDestinationInput) ([]repository.NotificationDestination, error)
}

func registerNotificationDestinationRoutes(mux *http.ServeMux, store InventoryStore) {
	backend, ok := store.(notificationDestinationStore)
	if !ok {
		return
	}
	base := "/api/v1/environments/{id}/notification-destinations"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		if !uuidPattern.MatchString(env) {
			writeError(w, http.StatusBadRequest, CodeValidation, "Ambiente inválido.")
			return
		}
		items, err := backend.ListNotificationDestinations(r.Context(), env)
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao listar destinatários.")
			return
		}
		if items == nil {
			items = []repository.NotificationDestination{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "using_global_fallback": len(items) == 0})
	})
	mux.HandleFunc("PUT "+base, func(w http.ResponseWriter, r *http.Request) {
		env := r.PathValue("id")
		var body struct {
			Items []repository.NotificationDestinationInput `json:"items"`
		}
		if !uuidPattern.MatchString(env) || json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&body) != nil {
			writeError(w, http.StatusBadRequest, CodeValidation, "Destinatários inválidos.")
			return
		}
		if body.Items == nil {
			body.Items = []repository.NotificationDestinationInput{}
		}
		actor := "operator"
		if user := requestIdentity(r); user != nil {
			actor = user.Username
		}
		items, err := backend.ReplaceNotificationDestinations(r.Context(), env, actor, body.Items)
		if err != nil {
			writeError(w, http.StatusBadRequest, CodeValidation, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "using_global_fallback": len(items) == 0})
	})
}

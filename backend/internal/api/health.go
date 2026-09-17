package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/mayconmendes-qc/timescale-auditor/internal/repository"
)

type readinessChecker interface {
	Ping(context.Context) error
}

type InventoryStore interface {
	readinessChecker
	ListEnvironmentsAPI(ctx context.Context) ([]repository.Environment, error)
	ListDatabaseSnapshots(ctx context.Context, environmentID string) ([]repository.DatabaseSnapshot, error)
	ListSchemaSnapshots(ctx context.Context, environmentID string) ([]repository.SchemaSnapshot, error)
}

func NewHandler(store InventoryStore) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health)
	mux.HandleFunc("GET /ready", ready(store))
	mux.HandleFunc("GET /api/v1/environments", listEnvironments(store))
	mux.HandleFunc("GET /api/v1/environments/{id}/databases", listDatabases(store))
	mux.HandleFunc("GET /api/v1/environments/{id}/schemas", listSchemas(store))
	return mux
}

func health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func ready(store readinessChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := store.Ping(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	}
}

func listEnvironments(store InventoryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := store.ListEnvironmentsAPI(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "list environments failed"})
			return
		}
		if items == nil {
			items = []repository.Environment{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func listDatabases(store InventoryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "environment id required"})
			return
		}
		items, err := store.ListDatabaseSnapshots(r.Context(), id)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "list databases failed"})
			return
		}
		if items == nil {
			items = []repository.DatabaseSnapshot{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func listSchemas(store InventoryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "environment id required"})
			return
		}
		items, err := store.ListSchemaSnapshots(r.Context(), id)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "list schemas failed"})
			return
		}
		if items == nil {
			items = []repository.SchemaSnapshot{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

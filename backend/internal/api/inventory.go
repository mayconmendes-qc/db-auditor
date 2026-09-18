package api

import (
	"net/http"

	"github.com/mayconmendes-qc/timescale-auditor/internal/repository"
)

func registerInventoryRoutes(mux *http.ServeMux, store InventoryStore) {
	mux.HandleFunc("GET /api/v1/environments/{id}/tables", listTables(store))
	mux.HandleFunc("GET /api/v1/environments/{id}/indexes", listIndexes(store))
	mux.HandleFunc("GET /api/v1/environments/{id}/views", listViews(store))
	mux.HandleFunc("GET /api/v1/environments/{id}/functions", listFunctions(store))
}

func listTables(store InventoryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "environment id required"})
			return
		}
		q := parseInventoryQuery(r)
		items, total, err := store.ListTableSnapshots(r.Context(), repository.InventoryFilter{
			EnvironmentID: id,
			Database:      q.Database,
			Schema:        q.Schema,
			Q:             q.Q,
			Limit:         q.Limit,
			Offset:        q.Offset,
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "list tables failed"})
			return
		}
		if items == nil {
			items = []repository.TableSnapshotRow{}
		}
		writePage(w, items, q.Limit, q.Offset, total)
	}
}

func listIndexes(store InventoryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "environment id required"})
			return
		}
		q := parseInventoryQuery(r)
		items, total, err := store.ListIndexSnapshots(r.Context(), repository.InventoryFilter{
			EnvironmentID: id,
			Database:      q.Database,
			Schema:        q.Schema,
			Q:             q.Q,
			Limit:         q.Limit,
			Offset:        q.Offset,
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "list indexes failed"})
			return
		}
		if items == nil {
			items = []repository.IndexSnapshotRow{}
		}
		writePage(w, items, q.Limit, q.Offset, total)
	}
}

func listViews(store InventoryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "environment id required"})
			return
		}
		q := parseInventoryQuery(r)
		items, total, err := store.ListViewSnapshots(r.Context(), repository.InventoryFilter{
			EnvironmentID: id,
			Database:      q.Database,
			Schema:        q.Schema,
			Q:             q.Q,
			Limit:         q.Limit,
			Offset:        q.Offset,
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "list views failed"})
			return
		}
		if items == nil {
			items = []repository.ViewSnapshotRow{}
		}
		writePage(w, items, q.Limit, q.Offset, total)
	}
}

func listFunctions(store InventoryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "environment id required"})
			return
		}
		q := parseInventoryQuery(r)
		items, total, err := store.ListFunctionSnapshots(r.Context(), repository.InventoryFilter{
			EnvironmentID: id,
			Database:      q.Database,
			Schema:        q.Schema,
			Q:             q.Q,
			Limit:         q.Limit,
			Offset:        q.Offset,
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "list functions failed"})
			return
		}
		if items == nil {
			items = []repository.FunctionSnapshotRow{}
		}
		writePage(w, items, q.Limit, q.Offset, total)
	}
}

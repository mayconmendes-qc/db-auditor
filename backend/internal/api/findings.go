package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/mayconmendes-qc/timescale-auditor/internal/analyzer"
	"github.com/mayconmendes-qc/timescale-auditor/internal/repository"
)

// FindingStore exposes finding persistence.
type FindingStore interface {
	ListFindings(ctx context.Context, environmentID, findingType, severity, status string, limit int) ([]repository.Finding, error)
	GetFinding(ctx context.Context, id string) (*repository.Finding, error)
	UpdateFindingStatus(ctx context.Context, id, status, notes string) (*repository.Finding, error)
	UpsertFinding(ctx context.Context, p repository.UpsertFindingParams) (*repository.Finding, error)
}

type updateFindingBody struct {
	Status string `json:"status"`
	Notes  string `json:"notes"`
}

type analyzeBody struct {
	EnvironmentID string                   `json:"environment_id"`
	AuditRunID    string                   `json:"audit_run_id"`
	Tables        []analyzer.TableFact     `json:"tables"`
	Indexes       []analyzer.IndexFact     `json:"indexes"`
	Hypertables   []analyzer.HypertableFact `json:"hypertables"`
	Chunks        []analyzer.ChunkFact     `json:"chunks"`
}

func registerFindingRoutes(mux *http.ServeMux, store FindingStore) {
	mux.HandleFunc("GET /api/v1/findings", listFindings(store))
	mux.HandleFunc("GET /api/v1/findings/{id}", getFinding(store))
	mux.HandleFunc("PATCH /api/v1/findings/{id}", patchFinding(store))
	mux.HandleFunc("POST /api/v1/findings/analyze", analyzeFindings(store))
}

func listFindings(store FindingStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		limit, _ := strconv.Atoi(q.Get("limit"))
		items, err := store.ListFindings(r.Context(), q.Get("environment_id"), q.Get("finding_type"), q.Get("severity"), q.Get("status"), limit)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "list findings failed"})
			return
		}
		if items == nil {
			items = []repository.Finding{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func getFinding(store FindingStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		f, err := store.GetFinding(r.Context(), id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "finding not found"})
			return
		}
		writeJSON(w, http.StatusOK, f)
	}
}

func patchFinding(store FindingStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var body updateFindingBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
			return
		}
		if body.Status == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "status required"})
			return
		}
		switch body.Status {
		case "open", "acknowledged", "resolved", "suppressed":
		default:
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid status"})
			return
		}
		f, err := store.UpdateFindingStatus(r.Context(), id, body.Status, body.Notes)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "finding not found"})
			return
		}
		writeJSON(w, http.StatusOK, f)
	}
}

func analyzeFindings(store FindingStore) http.HandlerFunc {
	runner := analyzer.NewRunner(analyzer.DefaultRegistry())
	return func(w http.ResponseWriter, r *http.Request) {
		var body analyzeBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
			return
		}
		if body.EnvironmentID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "environment_id required"})
			return
		}
		facts := analyzer.SnapshotFacts{
			EnvironmentID: body.EnvironmentID,
			AuditRunID:    body.AuditRunID,
			Tables:        body.Tables,
			Indexes:       body.Indexes,
			Hypertables:   body.Hypertables,
			Chunks:        body.Chunks,
		}
		produced, err := runner.Run(r.Context(), facts)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		saved := make([]repository.Finding, 0, len(produced))
		for _, f := range produced {
			row, err := store.UpsertFinding(r.Context(), repository.UpsertFindingParams{
				EnvironmentID: f.EnvironmentID,
				AuditRunID:    f.AuditRunID,
				FindingType:   f.FindingType,
				Severity:      string(f.Severity),
				Title:         f.Title,
				Summary:       f.Summary,
				ObjectType:    f.ObjectType,
				ObjectKey:     f.ObjectKey,
				DatabaseName:  f.DatabaseName,
				SchemaName:    f.SchemaName,
				ObjectName:    f.ObjectName,
				Evidence:      analyzer.EvidenceJSON(f.Evidence),
				DedupKey:      f.DedupKey,
			})
			if err != nil {
				// skip rows that conflict with suppressed/resolved
				continue
			}
			saved = append(saved, *row)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"produced": len(produced),
			"saved":    len(saved),
			"items":    saved,
		})
	}
}

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mayconmendes-qc/timescale-auditor/internal/repository"
)

type stubStore struct {
	pingErr error
}

func (s *stubStore) Ping(context.Context) error { return s.pingErr }

func (s *stubStore) ListEnvironmentsAPI(context.Context) ([]repository.Environment, error) {
	return []repository.Environment{}, nil
}
func (s *stubStore) ListDatabaseSnapshots(context.Context, string) ([]repository.DatabaseSnapshot, error) {
	return []repository.DatabaseSnapshot{}, nil
}
func (s *stubStore) ListSchemaSnapshots(context.Context, string) ([]repository.SchemaSnapshot, error) {
	return []repository.SchemaSnapshot{}, nil
}
func (s *stubStore) ListHypertableSnapshots(context.Context, string) ([]repository.HypertableSnapshotRow, error) {
	return []repository.HypertableSnapshotRow{}, nil
}
func (s *stubStore) ListDimensionSnapshots(context.Context, string) ([]repository.DimensionSnapshotRow, error) {
	return []repository.DimensionSnapshotRow{}, nil
}
func (s *stubStore) ListChunkSnapshots(context.Context, string) ([]repository.ChunkSnapshotRow, error) {
	return []repository.ChunkSnapshotRow{}, nil
}
func (s *stubStore) ListCAGGSnapshots(context.Context, string) ([]repository.CAGGSnapshotRow, error) {
	return []repository.CAGGSnapshotRow{}, nil
}
func (s *stubStore) ListJobSnapshots(context.Context, string) ([]repository.JobSnapshotRow, error) {
	return []repository.JobSnapshotRow{}, nil
}
func (s *stubStore) ListPolicySnapshots(context.Context, string) ([]repository.PolicySnapshotRow, error) {
	return []repository.PolicySnapshotRow{}, nil
}
func (s *stubStore) ListAuditRuns(context.Context, string, string, string, int) ([]repository.AuditRunRow, error) {
	return []repository.AuditRunRow{}, nil
}
func (s *stubStore) GetAuditRun(context.Context, string) (*repository.AuditRunRow, error) {
	return nil, nil
}
func (s *stubStore) ListCollectorRuns(context.Context, string) ([]repository.CollectorRunRow, error) {
	return []repository.CollectorRunRow{}, nil
}

func TestHealth(t *testing.T) {
	t.Parallel()
	h := NewHandler(&stubStore{})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Fatalf("body = %v", body)
	}
}

func TestListHypertablesEmpty(t *testing.T) {
	t.Parallel()
	h := NewHandler(&stubStore{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/environments/00000000-0000-0000-0000-000000000001/hypertables", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestListAuditRunsEmpty(t *testing.T) {
	t.Parallel()
	h := NewHandler(&stubStore{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-runs", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
}

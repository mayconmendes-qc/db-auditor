package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mayconmendes-qc/db-auditor/internal/repository"
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
func (s *stubStore) ListTableSnapshots(context.Context, repository.InventoryFilter) ([]repository.TableSnapshotRow, int, error) {
	return []repository.TableSnapshotRow{}, 0, nil
}
func (s *stubStore) ListColumnSnapshots(context.Context, repository.InventoryFilter) ([]repository.ColumnSnapshotRow, int, error) {
	return []repository.ColumnSnapshotRow{}, 0, nil
}
func (s *stubStore) ListIndexSnapshots(context.Context, repository.InventoryFilter) ([]repository.IndexSnapshotRow, int, error) {
	return []repository.IndexSnapshotRow{}, 0, nil
}
func (s *stubStore) ListViewSnapshots(context.Context, repository.InventoryFilter) ([]repository.ViewSnapshotRow, int, error) {
	return []repository.ViewSnapshotRow{}, 0, nil
}
func (s *stubStore) ListFunctionSnapshots(context.Context, repository.InventoryFilter) ([]repository.FunctionSnapshotRow, int, error) {
	return []repository.FunctionSnapshotRow{}, 0, nil
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
func (s *stubStore) GetSnapshotCompleteness(context.Context, string, string) (*repository.SnapshotCompleteness, error) {
	return &repository.SnapshotCompleteness{Completeness: "empty"}, nil
}
func (s *stubStore) ListObjectMappings(context.Context, string, string, string) ([]repository.ObjectMapping, error) {
	return []repository.ObjectMapping{}, nil
}
func (s *stubStore) CreateObjectMapping(context.Context, repository.CreateObjectMappingParams) (*repository.ObjectMapping, error) {
	return &repository.ObjectMapping{}, nil
}
func (s *stubStore) UpdateObjectMappingStatus(context.Context, string, string, string) (*repository.ObjectMapping, error) {
	return &repository.ObjectMapping{}, nil
}
func (s *stubStore) ListFindings(context.Context, string, string, string, string, int) ([]repository.Finding, error) {
	return []repository.Finding{}, nil
}
func (s *stubStore) GetFinding(context.Context, string) (*repository.Finding, error) {
	return nil, nil
}
func (s *stubStore) UpdateFindingStatus(context.Context, string, string, string) (*repository.Finding, error) {
	return &repository.Finding{}, nil
}
func (s *stubStore) UpsertFinding(context.Context, repository.UpsertFindingParams) (*repository.Finding, error) {
	return &repository.Finding{}, nil
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

func TestMetrics(t *testing.T) {
	t.Parallel()
	h := NewHandler(&stubStore{})
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "auditor_up 1") {
		t.Fatalf("metrics body: %s", w.Body.String())
	}
}

func TestStatus(t *testing.T) {
	t.Parallel()
	h := NewHandler(&stubStore{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["api_status"] != "ok" {
		t.Fatalf("body = %v", body)
	}
}

func TestListTablesEmpty(t *testing.T) {
	t.Parallel()
	h := NewHandler(&stubStore{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/environments/00000000-0000-0000-0000-000000000001/tables", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
}

func TestSnapshotStatus(t *testing.T) {
	t.Parallel()
	h := NewHandler(&stubStore{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/environments/00000000-0000-0000-0000-000000000001/snapshot-status", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
}

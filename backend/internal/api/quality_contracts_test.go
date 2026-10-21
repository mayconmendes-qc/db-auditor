package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/osmendes/db-auditor/internal/dqcontract"
	"github.com/osmendes/db-auditor/internal/repository"
)

type qualityContractStub struct {
	stubStore
	approved bool
	ran      bool
}

func (s *qualityContractStub) ListQualityContracts(context.Context, string) ([]repository.QualityContract, error) {
	return nil, nil
}
func (s *qualityContractStub) CreateQualityContract(context.Context, string, string, dqcontract.Document, string) (*repository.QualityContract, error) {
	return &repository.QualityContract{ID: "00000000-0000-4000-8000-0000000000c1"}, nil
}
func (s *qualityContractStub) AddQualityContractVersion(context.Context, string, string, string, dqcontract.Document, string) (*repository.QualityContractVersion, error) {
	return &repository.QualityContractVersion{ID: "00000000-0000-4000-8000-0000000000v1", Status: dqcontract.StatusProposed}, nil
}
func (s *qualityContractStub) ApproveQualityContractVersion(context.Context, string, string, string, string) (*repository.QualityContractVersion, error) {
	s.approved = true
	return &repository.QualityContractVersion{ID: "00000000-0000-4000-8000-0000000000v1", Status: dqcontract.StatusApproved}, nil
}
func (s *qualityContractStub) ProposeQualityContractFromSnapshot(context.Context, string, string, string, string, string) (*repository.QualityContract, error) {
	return nil, repository.ErrQualityContractNoSnapshot
}
func (s *qualityContractStub) ImportQualityContractYAML(context.Context, string, string, []byte) (*repository.QualityContract, error) {
	return &repository.QualityContract{ID: "00000000-0000-4000-8000-0000000000c1"}, nil
}
func (s *qualityContractStub) ExportQualityContractYAML(context.Context, string, string, string) (string, error) {
	return "database: db\n", nil
}
func (s *qualityContractStub) GetQualityContractVersion(context.Context, string, string, string) (*repository.QualityContractVersion, error) {
	return &repository.QualityContractVersion{ID: "00000000-0000-4000-8000-0000000000v1", Status: dqcontract.StatusProposed}, nil
}
func (s *qualityContractStub) ExecuteQualityContractRun(context.Context, string, string, string, string, string, string, dqcontract.Document, string, string) (*repository.QualityContractRun, error) {
	s.ran = true
	return &repository.QualityContractRun{ID: "00000000-0000-4000-8000-0000000000r1", Status: "success"}, nil
}
func (s *qualityContractStub) GetQualityContractRun(context.Context, string, string) (*repository.QualityContractRun, error) {
	return &repository.QualityContractRun{ID: "00000000-0000-4000-8000-0000000000r1"}, nil
}
func (s *qualityContractStub) ListQualityContractRuns(context.Context, string, string, int) ([]repository.QualityContractRunSummary, error) {
	return nil, nil
}
func (s *qualityContractStub) ListQualityContractTrend(context.Context, string, string, int) ([]repository.QualityContractTrendPoint, error) {
	return nil, nil
}

func TestViewerCannotApprove(t *testing.T) {
	env := "11111111-1111-1111-1111-111111111111"
	contract := "22222222-2222-2222-2222-222222222222"
	version := "33333333-3333-3333-3333-333333333333"
	store := &qualityContractStub{}
	auth := fakeAuthStore{user: &repository.AuditorUser{Role: "viewer", Environments: []string{env}}}
	h := NewHandlerWithOptions(store, HandlerOptions{Auth: auth})

	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/environments/"+env+"/quality-contracts/"+contract+"/versions/"+version+"/approve", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("CA-08: status=%d want 403 body=%s", rec.Code, rec.Body.String())
	}
	if store.approved {
		t.Fatal("CA-08: viewer must not reach ApproveQualityContractVersion")
	}

	approvePath := httptest.NewRequest(http.MethodPost,
		"/api/v1/environments/"+env+"/quality-contracts/"+contract+"/versions/"+version+"/approve", nil)
	viewer := &repository.AuditorUser{Role: "viewer", Environments: []string{env}}
	if _, ok := authorizeRequest(approvePath, viewer, fakeAuthStore{}); ok {
		t.Fatal("CA-08: authorizeRequest must deny viewer approve")
	}
	auditor := &repository.AuditorUser{Role: "auditor", Environments: []string{env}}
	if _, ok := authorizeRequest(approvePath, auditor, fakeAuthStore{}); !ok {
		t.Fatal("CA-08: auditor must be authorized to approve")
	}
}

func TestContractRunDisabled409AndCLIExit2(t *testing.T) {
	t.Setenv("AUDITOR_DATA_QUALITY_ENABLED", "")
	if dqcontract.DataQualityEnabled() {
		t.Fatal("expected DataQualityEnabled=false")
	}

	env := "11111111-1111-1111-1111-111111111111"
	contract := "22222222-2222-2222-2222-222222222222"
	store := &qualityContractStub{}
	mux := http.NewServeMux()
	registerQualityContractRoutes(mux, store, map[string]string{env: "postgres://readonly@db/app"})
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(context.WithValue(r.Context(), identityKey{}, &repository.AuditorUser{
			Username: "aud", Role: "auditor", Environments: []string{env},
		}))
		mux.ServeHTTP(w, r)
	})

	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/environments/"+env+"/quality-contracts/"+contract+"/runs",
		strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("CA-03: status=%d want 409 body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "desativado") {
		t.Fatalf("CA-03: expected disabled message, body=%s", rec.Body.String())
	}
	if store.ran {
		t.Fatal("CA-03: ExecuteQualityContractRun must not run when disabled")
	}

	// Ad-hoc runs path also gates on the same flag.
	req2 := httptest.NewRequest(http.MethodPost,
		"/api/v1/environments/"+env+"/quality-contracts/runs",
		strings.NewReader(`{"adhoc":true,"yaml":"database: db\nschema: public\ntable: t\nchecks: []\n"}`))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusConflict {
		t.Fatalf("CA-03 adhoc: status=%d want 409 body=%s", rec2.Code, rec2.Body.String())
	}
}

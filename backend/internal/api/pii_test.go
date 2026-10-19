package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/osmendes/db-auditor/internal/config"
	"github.com/osmendes/db-auditor/internal/repository"
)

type fakePIIStore struct {
	stubStore
	engine    string
	settings  *repository.PIIEnvironmentSetting
	createErr error
}

func (f *fakePIIStore) GetEnvironmentEngine(context.Context, string) (string, error) {
	if f.engine == "" {
		return "postgresql", nil
	}
	return f.engine, nil
}
func (f *fakePIIStore) GetPIISettings(_ context.Context, env string, process bool) (*repository.PIIEnvironmentSetting, error) {
	if f.settings != nil {
		s := *f.settings
		s.ProcessFlagEnabled = process
		s.EffectiveEnabled = process && s.ContentScanEnabled
		return &s, nil
	}
	return &repository.PIIEnvironmentSetting{EnvironmentID: env, ProcessFlagEnabled: process}, nil
}
func (f *fakePIIStore) PutPIISettings(context.Context, string, string, bool) (*repository.PIIEnvironmentSetting, error) {
	return f.settings, nil
}
func (f *fakePIIStore) ListPIIDictionary(context.Context, string) ([]repository.PIIDictionaryEntry, error) {
	return nil, nil
}
func (f *fakePIIStore) ReplacePIIDictionary(context.Context, string, string, []repository.PIIDictionaryEntry) ([]repository.PIIDictionaryEntry, error) {
	return nil, nil
}
func (f *fakePIIStore) CreatePIIScan(context.Context, string, string, []int, bool, string, config.Scope) (*repository.PIIScan, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	return &repository.PIIScan{ID: "00000000-0000-4000-8000-0000000000p1", Status: "success"}, nil
}
func (f *fakePIIStore) ListPIIScans(context.Context, string) ([]repository.PIIScan, error) {
	return nil, nil
}
func (f *fakePIIStore) GetPIIScan(context.Context, string, string) (*repository.PIIScan, error) {
	return &repository.PIIScan{ID: "00000000-0000-4000-8000-0000000000p1", Status: "success"}, nil
}
func (f *fakePIIStore) ListPIIMap(context.Context, string, repository.PIIMapFilter) ([]repository.PIIColumnResult, int, error) {
	return nil, 0, nil
}
func (f *fakePIIStore) ReviewPIIClassification(context.Context, string, string, string, string, string) (*repository.PIIClassificationReview, error) {
	return &repository.PIIClassificationReview{Decision: "confirmed"}, nil
}
func (f *fakePIIStore) StreamPIIMapCSV(_ context.Context, _ string, w io.Writer) (int, error) {
	_, _ = io.WriteString(w, "coluna\n# export_complete,0\n")
	return 0, nil
}
func (f *fakePIIStore) LogAuditorOperation(context.Context, *repository.AuditorUser, string, string, string, string) error {
	return nil
}

func TestContentStageDisabled409(t *testing.T) {
	t.Setenv("AUDITOR_PII_CONTENT_SCAN_ENABLED", "")
	store := &fakePIIStore{createErr: repository.ErrPIIContentDisabled}
	mux := http.NewServeMux()
	registerPIIRoutes(mux, store, nil)
	body, _ := json.Marshal(map[string]any{"stages": []int{1, 2}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/environments/00000000-0000-4000-8000-0000000000e1/pii-scans", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), identityKey{}, &repository.AuditorUser{Username: "aud", Role: "auditor"}))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("CA-05: want 409, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestPIIMongoNotApplicable(t *testing.T) {
	store := &fakePIIStore{engine: "mongodb"}
	mux := http.NewServeMux()
	registerPIIRoutes(mux, store, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/environments/00000000-0000-4000-8000-0000000000e1/pii-map?limit=20&offset=0", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 || !bytes.Contains(rec.Body.Bytes(), []byte("não aplicável")) {
		t.Fatalf("CA-06: %d %s", rec.Code, rec.Body.String())
	}
}

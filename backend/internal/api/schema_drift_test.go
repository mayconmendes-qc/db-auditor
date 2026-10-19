package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/osmendes/db-auditor/internal/comparison"
	"github.com/osmendes/db-auditor/internal/repository"
	"github.com/osmendes/db-auditor/internal/schemadiff"
)

type fakeSchemaDriftStore struct {
	stubStore
	diff       *repository.SchemaDiffResult
	diffErr    error
	gotReq     repository.SchemaDiffRequest
	timeline   *repository.SchemaTimelinePage
	registry   []repository.EnvironmentMigrationRegistry
	replaceErr error
	isProdSet  *bool
}

func (f *fakeSchemaDriftStore) BuildSchemaDiff(_ context.Context, req repository.SchemaDiffRequest) (*repository.SchemaDiffResult, error) {
	f.gotReq = req
	if f.diffErr != nil {
		return nil, f.diffErr
	}
	if f.diff != nil {
		return f.diff, nil
	}
	return &repository.SchemaDiffResult{
		SourceEnvironmentID: req.SourceEnv,
		TargetEnvironmentID: req.TargetEnv,
		SourceRunID:         "00000000-0000-0000-0000-0000000000aa",
		TargetRunID:         "00000000-0000-0000-0000-0000000000bb",
		Partial:             false,
		Objects:             []comparison.ObjectDiff{},
		Total:               0,
		Limit:               req.Limit,
		Offset:              req.Offset,
	}, nil
}

func (f *fakeSchemaDriftStore) ListSchemaTimeline(context.Context, string, time.Time, time.Time, int, int) (*repository.SchemaTimelinePage, error) {
	if f.timeline != nil {
		return f.timeline, nil
	}
	return &repository.SchemaTimelinePage{Events: []repository.SchemaChangeEventRow{}, Gaps: []repository.SchemaTimelineGapRow{}}, nil
}

func (f *fakeSchemaDriftStore) SetEnvironmentProduction(_ context.Context, _ string, isProd bool, _ string) error {
	f.isProdSet = &isProd
	return nil
}

func (f *fakeSchemaDriftStore) ListMigrationRegistry(context.Context, string) ([]repository.EnvironmentMigrationRegistry, error) {
	return f.registry, nil
}

func (f *fakeSchemaDriftStore) ReplaceMigrationRegistry(_ context.Context, env, _ string, inputs []repository.MigrationRegistryInput) ([]repository.EnvironmentMigrationRegistry, error) {
	if f.replaceErr != nil {
		return nil, f.replaceErr
	}
	out := make([]repository.EnvironmentMigrationRegistry, 0, len(inputs))
	for _, in := range inputs {
		out = append(out, repository.EnvironmentMigrationRegistry{
			EnvironmentID: env, Tool: in.Tool, SchemaName: in.SchemaName, TableName: in.TableName, Enabled: in.Enabled,
		})
	}
	f.registry = out
	return out, nil
}

func TestSchemaDiffCrossEnvironmentRequiresBothGrants(t *testing.T) {
	allowed := "00000000-0000-0000-0000-000000000001"
	other := "00000000-0000-0000-0000-000000000002"
	store := &fakeSchemaDriftStore{diff: &repository.SchemaDiffResult{
		Objects: []comparison.ObjectDiff{{ObjectType: "table", ObjectKey: "secret.from.b", Status: comparison.StatusOnlyTarget}},
		Total:   1,
	}}
	auth := fakeAuthStore{user: &repository.AuditorUser{Role: "viewer", Environments: []string{allowed}}}
	h := NewHandlerWithOptions(store, HandlerOptions{Auth: auth})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/environments/"+allowed+"/schema-diff?target_environment_id="+other, nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status %d want 403 body=%s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "secret.from.b") {
		t.Fatal("403 body must not reveal objects from B")
	}
}

func TestSchemaDiffMechanismMismatch400(t *testing.T) {
	env := "00000000-0000-0000-0000-000000000001"
	store := &fakeSchemaDriftStore{diffErr: schemadiff.ErrMechanismMismatch}
	mux := http.NewServeMux()
	registerSchemaDriftRoutes(mux, store)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/environments/"+env+"/schema-diff", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d want 400 body=%s", w.Code, w.Body.String())
	}
}

func TestSchemaDiffPartialFlag(t *testing.T) {
	env := "00000000-0000-0000-0000-000000000001"
	store := &fakeSchemaDriftStore{diff: &repository.SchemaDiffResult{
		SourceEnvironmentID: env, TargetEnvironmentID: env, Partial: true, Total: 0, Objects: []comparison.ObjectDiff{},
	}}
	mux := http.NewServeMux()
	registerSchemaDriftRoutes(mux, store)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/environments/"+env+"/schema-diff", nil))
	if w.Code != 200 {
		t.Fatalf("status %d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["partial"] != true {
		t.Fatalf("partial=%v", body["partial"])
	}
}

func TestSchemaDiffFunctionShowsFingerprintsOnly(t *testing.T) {
	env := "00000000-0000-0000-0000-000000000001"
	fpA := schemadiff.FingerprintValue("SELECT 'secret@example.com'")
	fpB := schemadiff.FingerprintValue("SELECT 1")
	store := &fakeSchemaDriftStore{diff: &repository.SchemaDiffResult{
		Objects: []comparison.ObjectDiff{{
			ObjectType: "function", ObjectKey: "db.public.f()", Status: comparison.StatusDrift,
			SourceFP: fpA, TargetFP: fpB,
			FieldDiffs: []comparison.FieldDiff{{Field: "definition_fp", Source: fpA, Target: fpB}},
		}},
		Total: 1,
	}}
	mux := http.NewServeMux()
	registerSchemaDriftRoutes(mux, store)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/environments/"+env+"/schema-diff", nil))
	body := w.Body.String()
	if strings.Contains(body, "secret@") || strings.Contains(body, "SELECT") {
		t.Fatalf("raw definition leaked: %s", body)
	}
	if !strings.Contains(body, fpA) || !strings.Contains(body, fpB) {
		t.Fatalf("fingerprints missing: %s", body)
	}
}

func TestMigrationRegistryRejectsMaliciousIdentifier(t *testing.T) {
	env := "00000000-0000-0000-0000-000000000001"
	store := &fakeSchemaDriftStore{}
	auth := fakeAuthStore{user: &repository.AuditorUser{Role: "operator", Environments: []string{env}}}
	h := NewHandlerWithOptions(store, HandlerOptions{Auth: auth})
	body := `{"items":[{"tool":"goose","schema_name":"x\"; drop","table_name":"t","enabled":true}]}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/environments/"+env+"/migration-registry", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer valid-token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d want 400 body=%s", w.Code, w.Body.String())
	}
}

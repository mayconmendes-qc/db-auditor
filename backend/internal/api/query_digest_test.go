package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/osmendes/db-auditor/internal/repository"
)

type fakeDigestStore struct {
	InventoryStore
	digests map[string]*repository.QueryDigest
	cfg     *repository.QueryDigestConfig
	dests   []repository.NotificationDestination
	engine  string
}

func (f *fakeDigestStore) Ping(context.Context) error { return nil }
func (f *fakeDigestStore) ListEnvironmentsAPI(context.Context) ([]repository.Environment, error) {
	return nil, nil
}
func (f *fakeDigestStore) ListDatabaseSnapshots(context.Context, string) ([]repository.DatabaseSnapshot, error) {
	return nil, nil
}
func (f *fakeDigestStore) ListSchemaSnapshots(context.Context, string) ([]repository.SchemaSnapshot, error) {
	return nil, nil
}
func (f *fakeDigestStore) ListHypertableSnapshots(context.Context, string) ([]repository.HypertableSnapshotRow, error) {
	return nil, nil
}
func (f *fakeDigestStore) ListHypertableSnapshotsPage(context.Context, repository.InventoryFilter) ([]repository.HypertableSnapshotRow, int, error) {
	return nil, 0, nil
}
func (f *fakeDigestStore) ListDimensionSnapshots(context.Context, string) ([]repository.DimensionSnapshotRow, error) {
	return nil, nil
}
func (f *fakeDigestStore) ListChunkSnapshots(context.Context, string) ([]repository.ChunkSnapshotRow, error) {
	return nil, nil
}
func (f *fakeDigestStore) ListCAGGSnapshots(context.Context, string) ([]repository.CAGGSnapshotRow, error) {
	return nil, nil
}
func (f *fakeDigestStore) ListCAGGSnapshotsPage(context.Context, repository.InventoryFilter) ([]repository.CAGGSnapshotRow, int, error) {
	return nil, 0, nil
}
func (f *fakeDigestStore) ListJobSnapshots(context.Context, string) ([]repository.JobSnapshotRow, error) {
	return nil, nil
}
func (f *fakeDigestStore) ListPolicySnapshots(context.Context, string) ([]repository.PolicySnapshotRow, error) {
	return nil, nil
}
func (f *fakeDigestStore) ListTableSnapshots(context.Context, repository.InventoryFilter) ([]repository.TableSnapshotRow, int, error) {
	return nil, 0, nil
}
func (f *fakeDigestStore) ListColumnSnapshots(context.Context, repository.InventoryFilter) ([]repository.ColumnSnapshotRow, int, error) {
	return nil, 0, nil
}
func (f *fakeDigestStore) ListIndexSnapshots(context.Context, repository.InventoryFilter) ([]repository.IndexSnapshotRow, int, error) {
	return nil, 0, nil
}
func (f *fakeDigestStore) ListViewSnapshots(context.Context, repository.InventoryFilter) ([]repository.ViewSnapshotRow, int, error) {
	return nil, 0, nil
}
func (f *fakeDigestStore) ListFunctionSnapshots(context.Context, repository.InventoryFilter) ([]repository.FunctionSnapshotRow, int, error) {
	return nil, 0, nil
}
func (f *fakeDigestStore) ListConstraintSnapshots(context.Context, repository.InventoryFilter) ([]repository.ConstraintSnapshotRow, int, error) {
	return nil, 0, nil
}
func (f *fakeDigestStore) ListAuditRuns(context.Context, string, string, string, int) ([]repository.AuditRunRow, error) {
	return nil, nil
}
func (f *fakeDigestStore) GetAuditRun(context.Context, string) (*repository.AuditRunRow, error) {
	return nil, nil
}
func (f *fakeDigestStore) ListCollectorRuns(context.Context, string) ([]repository.CollectorRunRow, error) {
	return nil, nil
}
func (f *fakeDigestStore) ListAuditRunCoverage(context.Context, string) ([]repository.AuditRunCoverage, error) {
	return nil, nil
}
func (f *fakeDigestStore) GetAnalysisRun(context.Context, string) (*repository.AnalysisRun, error) {
	return nil, nil
}
func (f *fakeDigestStore) GetSnapshotCompleteness(context.Context, string, string) (*repository.SnapshotCompleteness, error) {
	return nil, nil
}
func (f *fakeDigestStore) ListObjectMappings(context.Context, string, string, string) ([]repository.ObjectMapping, error) {
	return nil, nil
}
func (f *fakeDigestStore) CreateObjectMapping(context.Context, repository.CreateObjectMappingParams) (*repository.ObjectMapping, error) {
	return nil, nil
}
func (f *fakeDigestStore) UpdateObjectMappingStatus(context.Context, string, string, string) (*repository.ObjectMapping, error) {
	return nil, nil
}
func (f *fakeDigestStore) ListFindings(context.Context, string, string, string, string, int) ([]repository.Finding, error) {
	return nil, nil
}
func (f *fakeDigestStore) CountFindings(context.Context, string) (repository.FindingAggregate, error) {
	return repository.FindingAggregate{}, nil
}
func (f *fakeDigestStore) CountRecentRuns(context.Context, string) (int, int, error) {
	return 0, 0, nil
}
func (f *fakeDigestStore) CountLatestCapabilities(context.Context, string) (repository.CapabilityAggregate, error) {
	return repository.CapabilityAggregate{}, nil
}
func (f *fakeDigestStore) ListStorageTrend(context.Context, string, time.Time, time.Time, string) ([]repository.RunTrendPoint, error) {
	return nil, nil
}
func (f *fakeDigestStore) ListFindingTrend(context.Context, string, time.Time, time.Time, string) ([]repository.RunTrendPoint, error) {
	return nil, nil
}
func (f *fakeDigestStore) ListTopTableConsumers(context.Context, string, int) ([]repository.StorageConsumer, error) {
	return nil, nil
}
func (f *fakeDigestStore) GetFinding(context.Context, string) (*repository.Finding, error) {
	return nil, nil
}
func (f *fakeDigestStore) UpdateFindingStatus(context.Context, string, string, string) (*repository.Finding, error) {
	return nil, nil
}
func (f *fakeDigestStore) UpsertFinding(context.Context, repository.UpsertFindingParams) (*repository.Finding, error) {
	return nil, nil
}

func (f *fakeDigestStore) ListQueryDigests(_ context.Context, env string, _, _ int) ([]repository.QueryDigest, int, error) {
	var out []repository.QueryDigest
	for _, d := range f.digests {
		if d.EnvironmentID == env {
			out = append(out, *d)
		}
	}
	return out, len(out), nil
}
func (f *fakeDigestStore) GetQueryDigest(_ context.Context, env, id string) (*repository.QueryDigest, error) {
	d := f.digests[id]
	if d == nil || d.EnvironmentID != env {
		return nil, nil
	}
	return d, nil
}
func (f *fakeDigestStore) GenerateQueryDigest(_ context.Context, env string, _ time.Time) (*repository.QueryDigest, error) {
	if f.engine == "mongodb" {
		d := &repository.QueryDigest{
			ID: "00000000-0000-0000-0000-000000000099", EnvironmentID: env,
			State: "not_applicable", Reason: "não aplicável a este mecanismo",
			Items: []repository.QueryDigestItem{}, Deliveries: []repository.QueryDigestDelivery{},
		}
		f.digests[d.ID] = d
		return d, nil
	}
	d := &repository.QueryDigest{
		ID: "00000000-0000-0000-0000-000000000098", EnvironmentID: env, State: "ok",
		Items: []repository.QueryDigestItem{}, Deliveries: []repository.QueryDigestDelivery{},
	}
	f.digests[d.ID] = d
	return d, nil
}
func (f *fakeDigestStore) GetQueryDigestConfig(_ context.Context, env string) (*repository.QueryDigestConfig, error) {
	if f.cfg == nil {
		return &repository.QueryDigestConfig{EnvironmentID: env, WindowDays: 7, Thresholds: json.RawMessage(`{}`), DeliveryEnabled: true}, nil
	}
	return f.cfg, nil
}
func (f *fakeDigestStore) UpsertQueryDigestConfig(_ context.Context, env string, days int, th json.RawMessage, delivery bool, actor string) (*repository.QueryDigestConfig, error) {
	f.cfg = &repository.QueryDigestConfig{EnvironmentID: env, WindowDays: days, Thresholds: th, DeliveryEnabled: delivery, UpdatedBy: actor}
	return f.cfg, nil
}
func (f *fakeDigestStore) DeliverQueryDigest(context.Context, *repository.QueryDigest) error {
	return nil
}
func (f *fakeDigestStore) ListNotificationDestinations(context.Context, string) ([]repository.NotificationDestination, error) {
	return f.dests, nil
}
func (f *fakeDigestStore) ReplaceNotificationDestinations(_ context.Context, env, _ string, inputs []repository.NotificationDestinationInput) ([]repository.NotificationDestination, error) {
	for _, in := range inputs {
		if err := repository.ValidateNotificationDestination(in); err != nil {
			return nil, err
		}
	}
	f.dests = nil
	for _, in := range inputs {
		f.dests = append(f.dests, repository.NotificationDestination{
			ID: "00000000-0000-0000-0000-000000000077", EnvironmentID: env,
			Channel: in.Channel, TargetMasked: repository.MaskDestination(in.Channel, in.Target),
			Format: in.Format, Active: in.Active,
		})
	}
	return f.dests, nil
}

func TestDigestConfigRequiresOperator(t *testing.T) {
	env := "00000000-0000-0000-0000-000000000001"
	store := &fakeDigestStore{digests: map[string]*repository.QueryDigest{}}
	for _, tc := range []struct {
		role, method string
		want         int
	}{
		{"viewer", http.MethodPut, 403},
		{"auditor", http.MethodPut, 403},
		{"operator", http.MethodPut, 200},
		{"viewer", http.MethodPost, 403},
		{"operator", http.MethodPost, 201},
	} {
		auth := fakeAuthStore{user: &repository.AuditorUser{Role: tc.role, Environments: []string{env}}}
		h := NewHandlerWithOptions(store, HandlerOptions{Auth: auth})
		var req *http.Request
		if tc.method == http.MethodPut {
			req = httptest.NewRequest(tc.method, "/api/v1/environments/"+env+"/query-digest-config", strings.NewReader(`{"window_days":7,"delivery_enabled":true}`))
		} else {
			req = httptest.NewRequest(tc.method, "/api/v1/environments/"+env+"/query-digests", strings.NewReader(`{}`))
		}
		req.Header.Set("Authorization", "Bearer valid-token")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != tc.want {
			t.Fatalf("%s %s: status %d want %d body=%s", tc.role, tc.method, w.Code, tc.want, w.Body.String())
		}
	}
}

func TestNotificationDestinationsRequireOperator(t *testing.T) {
	env := "00000000-0000-0000-0000-000000000001"
	store := &fakeDigestStore{digests: map[string]*repository.QueryDigest{}}
	for _, tc := range []struct {
		role, method string
		want         int
	}{
		{"viewer", http.MethodGet, 403},
		{"auditor", http.MethodGet, 403},
		{"operator", http.MethodGet, 200},
		{"viewer", http.MethodPut, 403},
		{"auditor", http.MethodPut, 403},
		{"operator", http.MethodPut, 200},
	} {
		auth := fakeAuthStore{user: &repository.AuditorUser{Role: tc.role, Environments: []string{env}}}
		h := NewHandlerWithOptions(store, HandlerOptions{Auth: auth})
		var body *bytes.Reader
		if tc.method == http.MethodPut {
			body = bytes.NewReader([]byte(`{"items":[{"channel":"email","target":"ops@example.com","format":"generic","active":true}]}`))
		} else {
			body = bytes.NewReader(nil)
		}
		req := httptest.NewRequest(tc.method, "/api/v1/environments/"+env+"/notification-destinations", body)
		req.Header.Set("Authorization", "Bearer valid-token")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != tc.want {
			t.Fatalf("%s %s: status %d want %d body=%s", tc.role, tc.method, w.Code, tc.want, w.Body.String())
		}
	}
}

func TestWebhookURLValidationAndNeverExposed(t *testing.T) {
	env := "00000000-0000-0000-0000-000000000001"
	store := &fakeDigestStore{digests: map[string]*repository.QueryDigest{}}
	auth := fakeAuthStore{user: &repository.AuditorUser{Role: "operator", Environments: []string{env}}}
	h := NewHandlerWithOptions(store, HandlerOptions{Auth: auth})

	for _, bad := range []string{
		`{"items":[{"channel":"webhook","target":"http://example.com/hook","format":"generic","active":true}]}`,
		`{"items":[{"channel":"webhook","target":"https://user:pass@example.com/hook","format":"generic","active":true}]}`,
		`{"items":[{"channel":"webhook","target":"https://127.0.0.1/hook","format":"generic","active":true}]}`,
		`{"items":[{"channel":"webhook","target":"https://10.0.0.5/hook","format":"generic","active":true}]}`,
		`{"items":[{"channel":"webhook","target":"https://hooks.example.com/hook","format":"slack","active":true}]}`,
	} {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/environments/"+env+"/notification-destinations", strings.NewReader(bad))
		req.Header.Set("Authorization", "Bearer valid-token")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 400 {
			t.Fatalf("want 400 for %s got %d", bad, w.Code)
		}
	}

	good := `{"items":[{"channel":"webhook","target":"https://hooks.example.com/secret-token-xyz","format":"generic","active":true}]}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/environments/"+env+"/notification-destinations", strings.NewReader(good))
	req.Header.Set("Authorization", "Bearer valid-token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status %d body=%s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "secret-token-xyz") {
		t.Fatal("full webhook URL leaked in response")
	}
	if !strings.Contains(w.Body.String(), "hooks.example.com") {
		t.Fatal("expected masked host in response")
	}
}

func TestDigestMongoNotApplicable(t *testing.T) {
	env := "00000000-0000-0000-0000-000000000001"
	store := &fakeDigestStore{digests: map[string]*repository.QueryDigest{}, engine: "mongodb"}
	auth := fakeAuthStore{user: &repository.AuditorUser{Role: "operator", Environments: []string{env}}}
	h := NewHandlerWithOptions(store, HandlerOptions{Auth: auth})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/environments/"+env+"/query-digests", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer valid-token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("status %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "not_applicable") {
		t.Fatalf("body=%s", w.Body.String())
	}
}

func TestValidateWebhookHelpers(t *testing.T) {
	if err := repository.ValidateNotificationDestination(repository.NotificationDestinationInput{
		Channel: "webhook", Target: "https://example.com/a", Format: "generic", Active: true,
	}); err != nil {
		t.Fatal(err)
	}
	masked := repository.MaskDestination("webhook", "https://hooks.slack.com/services/T/B/xxx")
	if strings.Contains(masked, "xxx") || !strings.Contains(masked, "hooks.slack.com") {
		t.Fatalf("masked=%s", masked)
	}
}

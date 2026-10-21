package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/osmendes/db-auditor/internal/digest"
	"github.com/osmendes/db-auditor/internal/notify"
	"github.com/osmendes/db-auditor/internal/repository"
)

type fakeOpenAlert struct {
	ID, EnvironmentID, Category, Status string
	Keys                                []string
}

type fakeDigestDelivery struct {
	DigestID, EnvironmentID, Channel, TargetMasked, Status, Error string
	DestinationID                                                 string
	Global                                                        bool
	Body                                                          string
}

type fakeDigestStore struct {
	InventoryStore
	digests map[string]*repository.QueryDigest
	cfg     *repository.QueryDigestConfig
	// dests kept for backward-compat with existing destination tests (single-env).
	dests []repository.NotificationDestination
	// destsByEnv is used when set (isolation / multi-env delivery).
	destsByEnv map[string][]repository.NotificationDestination
	rawTargets map[string]string // destination id → full target
	engine     string

	openAlerts          []fakeOpenAlert
	regressionAlertRows int // counted; GenerateQueryDigest must never increment
	seedFPByEnv         map[string]string
	nextDigestSeq       int

	mu         sync.Mutex
	deliveries []fakeDigestDelivery
	claimed    map[string]bool // env|digest → ClaimAlert success
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
	if f.digests == nil {
		f.digests = map[string]*repository.QueryDigest{}
	}
	if f.engine == "mongodb" {
		d := &repository.QueryDigest{
			ID: "00000000-0000-0000-0000-000000000099", EnvironmentID: env,
			State: "not_applicable", Reason: "não aplicável a este mecanismo",
			Items: []repository.QueryDigestItem{}, Deliveries: []repository.QueryDigestDelivery{},
		}
		f.digests[d.ID] = d
		return d, nil
	}
	f.nextDigestSeq++
	id := fmt.Sprintf("00000000-0000-0000-0000-%012d", 90+f.nextDigestSeq)
	fp := "fp-" + env[len(env)-4:]
	if f.seedFPByEnv != nil {
		if seeded, ok := f.seedFPByEnv[env]; ok {
			fp = seeded
		}
	}
	var alertID *string
	for _, a := range f.openAlerts {
		if a.EnvironmentID != env || a.Category != "workload" || a.Status != "open" {
			continue
		}
		for _, k := range a.Keys {
			if k == fp {
				idCopy := a.ID
				alertID = &idCopy
				break
			}
		}
		if alertID != nil {
			break
		}
	}
	// CA-04 / RF-08: link only — never create regression_alert rows.
	item := repository.QueryDigestItem{
		ID: "00000000-0000-0000-0000-000000000050", DigestID: id,
		Classification: digest.ClassRegression, Severity: digest.SeverityMedium,
		Fingerprint: fp, DatabaseName: "app", ReferencedObjects: []string{"public.orders"},
		PriorCalls: 100, CurrentCalls: 200, PriorMeanMS: 20, CurrentMeanMS: 35,
		MeanChangePct: 0.75, RegressionAlertID: alertID, RelatedFindingIDs: []string{},
	}
	md := digest.RenderMarkdown(digest.MarkdownInput{
		EnvironmentName: "env", EnvironmentID: env, State: "ok", WorkloadLimitN: 500,
		Sections: digest.DigestSections{Regressions: []digest.ClassifiedItem{{
			Classification: item.Classification, Severity: item.Severity, Fingerprint: fp,
			DatabaseName: "app", ReferencedObjects: item.ReferencedObjects,
			PriorCalls: item.PriorCalls, CurrentCalls: item.CurrentCalls,
			PriorMeanMS: item.PriorMeanMS, CurrentMeanMS: item.CurrentMeanMS, MeanChangePct: item.MeanChangePct,
		}}},
	})
	d := &repository.QueryDigest{
		ID: id, EnvironmentID: env, State: "ok", Markdown: md,
		RegressionCount: 1, Items: []repository.QueryDigestItem{item},
		Deliveries: []repository.QueryDigestDelivery{},
	}
	f.digests[d.ID] = d
	return d, nil
}
func (f *fakeDigestStore) GetQueryDigestConfig(_ context.Context, env string) (*repository.QueryDigestConfig, error) {
	if f.cfg == nil {
		return &repository.QueryDigestConfig{EnvironmentID: env, WindowDays: 7, Thresholds: json.RawMessage(`{}`), DeliveryEnabled: true}, nil
	}
	cfg := *f.cfg
	cfg.EnvironmentID = env
	return &cfg, nil
}
func (f *fakeDigestStore) UpsertQueryDigestConfig(_ context.Context, env string, days int, th json.RawMessage, delivery bool, actor string) (*repository.QueryDigestConfig, error) {
	f.cfg = &repository.QueryDigestConfig{EnvironmentID: env, WindowDays: days, Thresholds: th, DeliveryEnabled: delivery, UpdatedBy: actor}
	return f.cfg, nil
}
func (f *fakeDigestStore) DeliverQueryDigest(ctx context.Context, d *repository.QueryDigest) error {
	if d == nil {
		return nil
	}
	f.mu.Lock()
	if f.claimed == nil {
		f.claimed = map[string]bool{}
	}
	key := d.EnvironmentID + "|" + d.ID
	if f.claimed[key] {
		f.mu.Unlock()
		return nil
	}
	f.mu.Unlock()

	dests := f.activeDests(d.EnvironmentID)
	event := notify.Event{
		Kind: "query_digest", EnvironmentID: d.EnvironmentID, DedupKey: d.ID,
		Title: fmt.Sprintf("Digest de consultas (%s)", d.State), Severity: "info",
	}
	payload := notify.DigestPayload{
		Event: event, Link: "/digest?env=" + d.EnvironmentID + "&id=" + d.ID,
		RegressionCount: d.RegressionCount, NewCount: d.NewCount,
		GoneCount: d.GoneCount, ImprovementCount: d.ImprovementCount, State: d.State,
	}
	allOK := true
	if len(dests) == 0 {
		cfg := notify.FromEnv()
		if !cfg.Enabled() {
			return nil
		}
		body, _ := json.Marshal(event)
		status, errStr := "sent", ""
		if err := cfg.Send(ctx, event); err != nil {
			allOK = false
			status, errStr = "failed", err.Error()
		}
		f.recordDelivery(fakeDigestDelivery{
			DigestID: d.ID, EnvironmentID: d.EnvironmentID, Channel: "global",
			TargetMasked: "global", Status: status, Error: errStr, Global: true, Body: string(body),
		})
	} else {
		notifyDests := make([]notify.Destination, 0, len(dests))
		for _, dest := range dests {
			target := dest.TargetMasked
			if f.rawTargets != nil {
				if raw, ok := f.rawTargets[dest.ID]; ok {
					target = raw
				}
			}
			notifyDests = append(notifyDests, notify.Destination{
				ID: dest.ID, Channel: dest.Channel, Target: target, Format: dest.Format,
			})
		}
		errs := notify.SendToDestinations(ctx, notifyDests, event, payload)
		for i, dest := range dests {
			status, errStr := "sent", ""
			var body string
			if raw, err := json.Marshal(payload); err == nil {
				body = string(raw)
			}
			if i < len(errs) && errs[i] != nil {
				allOK = false
				status, errStr = "failed", errs[i].Error()
			}
			f.recordDelivery(fakeDigestDelivery{
				DigestID: d.ID, EnvironmentID: d.EnvironmentID, DestinationID: dest.ID,
				Channel: dest.Channel, TargetMasked: dest.TargetMasked, Status: status, Error: errStr, Body: body,
			})
		}
	}
	if allOK {
		f.mu.Lock()
		f.claimed[key] = true
		f.mu.Unlock()
	}
	if d != nil {
		f.mu.Lock()
		if stored := f.digests[d.ID]; stored != nil {
			stored.Deliveries = f.deliveriesFor(d.ID)
		}
		f.mu.Unlock()
	}
	return nil
}

func (f *fakeDigestStore) RetryFailedQueryDigestDeliveries(ctx context.Context, env string) error {
	f.mu.Lock()
	var ids []string
	seen := map[string]bool{}
	for _, del := range f.deliveries {
		if del.EnvironmentID == env && del.Status == "failed" && !seen[del.DigestID] {
			if f.claimed != nil && f.claimed[env+"|"+del.DigestID] {
				continue
			}
			seen[del.DigestID] = true
			ids = append(ids, del.DigestID)
		}
	}
	f.mu.Unlock()
	for _, id := range ids {
		d, err := f.GetQueryDigest(ctx, env, id)
		if err != nil || d == nil {
			continue
		}
		_ = f.DeliverQueryDigest(ctx, d)
	}
	return nil
}

func (f *fakeDigestStore) activeDests(env string) []repository.NotificationDestination {
	if f.destsByEnv != nil {
		var out []repository.NotificationDestination
		for _, d := range f.destsByEnv[env] {
			if d.Active {
				out = append(out, d)
			}
		}
		return out
	}
	var out []repository.NotificationDestination
	for _, d := range f.dests {
		if d.EnvironmentID == env && d.Active {
			out = append(out, d)
		}
	}
	return out
}

func (f *fakeDigestStore) recordDelivery(d fakeDigestDelivery) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deliveries = append(f.deliveries, d)
}

func (f *fakeDigestStore) deliveriesFor(digestID string) []repository.QueryDigestDelivery {
	var out []repository.QueryDigestDelivery
	for _, d := range f.deliveries {
		if d.DigestID != digestID {
			continue
		}
		var destID *string
		if d.DestinationID != "" {
			id := d.DestinationID
			destID = &id
		}
		out = append(out, repository.QueryDigestDelivery{
			DigestID: d.DigestID, DestinationID: destID, Channel: d.Channel,
			TargetMasked: d.TargetMasked, Status: d.Status, Attempt: 1, ErrorSanitized: d.Error,
		})
	}
	return out
}

func (f *fakeDigestStore) ListNotificationDestinations(_ context.Context, env string) ([]repository.NotificationDestination, error) {
	if f.destsByEnv != nil {
		items := f.destsByEnv[env]
		if items == nil {
			return []repository.NotificationDestination{}, nil
		}
		return items, nil
	}
	var out []repository.NotificationDestination
	for _, d := range f.dests {
		if d.EnvironmentID == env || d.EnvironmentID == "" {
			out = append(out, d)
		}
	}
	return out, nil
}
func (f *fakeDigestStore) ReplaceNotificationDestinations(_ context.Context, env, _ string, inputs []repository.NotificationDestinationInput) ([]repository.NotificationDestination, error) {
	for _, in := range inputs {
		if err := repository.ValidateNotificationDestination(in); err != nil {
			return nil, err
		}
	}
	if f.rawTargets == nil {
		f.rawTargets = map[string]string{}
	}
	out := make([]repository.NotificationDestination, 0, len(inputs))
	for i, in := range inputs {
		id := fmt.Sprintf("00000000-0000-0000-0000-%012d", 70+i)
		dest := repository.NotificationDestination{
			ID: id, EnvironmentID: env,
			Channel: in.Channel, TargetMasked: repository.MaskDestination(in.Channel, in.Target),
			Format: in.Format, Active: in.Active,
		}
		out = append(out, dest)
		f.rawTargets[id] = in.Target
	}
	if f.destsByEnv != nil {
		f.destsByEnv[env] = out
	} else {
		f.dests = out
	}
	return out, nil
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
		`{"items":[{"channel":"webhook","target":"https://hooks.example.com/hook","format":"xml","active":true}]}`,
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

// TestDigestLinksOpenWorkloadAlert — CA-04 / RF-08: item links open workload alert;
// generating a digest must not create new regression_alert rows.
func TestDigestLinksOpenWorkloadAlert(t *testing.T) {
	env := "00000000-0000-0000-0000-000000000001"
	alertID := "00000000-0000-0000-0000-0000000000aa"
	fp := "fp-reg-shared"
	store := &fakeDigestStore{
		digests:             map[string]*repository.QueryDigest{},
		regressionAlertRows: 1,
		seedFPByEnv:         map[string]string{env: fp},
		openAlerts: []fakeOpenAlert{{
			ID: alertID, EnvironmentID: env, Category: "workload", Status: "open", Keys: []string{fp},
		}},
	}
	before := store.regressionAlertRows
	auth := fakeAuthStore{user: &repository.AuditorUser{Role: "operator", Environments: []string{env}}}
	h := NewHandlerWithOptions(store, HandlerOptions{Auth: auth})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/environments/"+env+"/query-digests", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer valid-token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("status %d body=%s", w.Code, w.Body.String())
	}
	if store.regressionAlertRows != before {
		t.Fatalf("regression_alert rows changed: before=%d after=%d", before, store.regressionAlertRows)
	}
	var body repository.QueryDigest
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Items) == 0 {
		t.Fatal("expected digest items")
	}
	linked := false
	for _, it := range body.Items {
		if it.RegressionAlertID != nil && *it.RegressionAlertID == alertID {
			linked = true
		}
	}
	if !linked {
		t.Fatalf("item did not link open workload alert: %+v", body.Items)
	}
	if strings.Contains(body.Markdown, "SELECT ") || strings.Contains(body.Markdown, "INSERT INTO") {
		t.Fatal("markdown must not contain SQL literals (CA-05)")
	}
}

// TestDigestRecipientsPerEnvironmentWithGlobalFallback — D6 / RF-07:
// env with destinations uses them; env without falls back to global channels.
func TestDigestRecipientsPerEnvironmentWithGlobalFallback(t *testing.T) {
	envA := "00000000-0000-0000-0000-00000000000a"
	envB := "00000000-0000-0000-0000-00000000000b"
	var hitsA, hitsGlobal atomic.Int32
	srvA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitsA.Add(1)
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "SELECT ") || strings.Contains(string(body), "'secret'") {
			t.Errorf("webhook body leaked SQL: %s", body)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srvA.Close()
	srvGlobal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitsGlobal.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srvGlobal.Close()

	t.Setenv("AUDITOR_ALERT_WEBHOOK_URL", srvGlobal.URL)
	t.Setenv("AUDITOR_ALERT_EMAIL_TO", "")
	t.Setenv("AUDITOR_ALERT_SMTP_HOST", "")

	store := &fakeDigestStore{
		digests:    map[string]*repository.QueryDigest{},
		destsByEnv: map[string][]repository.NotificationDestination{},
		rawTargets: map[string]string{},
		cfg:        &repository.QueryDigestConfig{WindowDays: 7, DeliveryEnabled: true, Thresholds: json.RawMessage(`{}`)},
	}
	destID := "00000000-0000-0000-0000-000000000071"
	store.destsByEnv[envA] = []repository.NotificationDestination{{
		ID: destID, EnvironmentID: envA, Channel: "webhook",
		TargetMasked: repository.MaskDestination("webhook", srvA.URL),
		Format:       "generic", Active: true,
	}}
	store.rawTargets[destID] = srvA.URL

	auth := fakeAuthStore{user: &repository.AuditorUser{Role: "operator", Environments: []string{envA, envB}}}
	h := NewHandlerWithOptions(store, HandlerOptions{Auth: auth})

	// Destinations API: A has dests (no global fallback flag); B empty → using_global_fallback.
	for _, tc := range []struct {
		env    string
		wantFB bool
	}{{envA, false}, {envB, true}} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/environments/"+tc.env+"/notification-destinations", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("%s destinations status %d", tc.env, w.Code)
		}
		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		if fb, _ := resp["using_global_fallback"].(bool); fb != tc.wantFB {
			t.Fatalf("%s using_global_fallback=%v want %v", tc.env, fb, tc.wantFB)
		}
	}

	postDigest := func(env string) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/environments/"+env+"/query-digests", strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer valid-token")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("generate %s: %d %s", env, w.Code, w.Body.String())
		}
	}
	postDigest(envA)
	postDigest(envB)

	if hitsA.Load() < 1 {
		t.Fatal("env A destination webhook was not hit")
	}
	if hitsGlobal.Load() < 1 {
		t.Fatal("env B without destinations must use global webhook fallback")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	for _, d := range store.deliveries {
		if d.EnvironmentID == envA && d.Global {
			t.Fatal("env A with destinations must not use global fallback")
		}
		if d.EnvironmentID == envB && !d.Global {
			t.Fatal("env B without destinations must record global delivery")
		}
	}
}

// TestDigestNeverMixesEnvironments — RNF-04: digest/delivery for A never uses B data/destinations;
// viewer without grant to B gets 403.
func TestDigestNeverMixesEnvironments(t *testing.T) {
	envA := "00000000-0000-0000-0000-00000000000a"
	envB := "00000000-0000-0000-0000-00000000000b"
	fpA, fpB := "fingerprint-only-a", "fingerprint-only-b"

	var gotBodies []string
	var mu sync.Mutex
	srvB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		gotBodies = append(gotBodies, string(b))
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srvB.Close()
	srvA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		gotBodies = append(gotBodies, string(b))
		mu.Unlock()
		if strings.Contains(string(b), fpB) || strings.Contains(string(b), envB) {
			t.Errorf("A webhook received B data: %s", b)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srvA.Close()

	t.Setenv("AUDITOR_ALERT_WEBHOOK_URL", "")
	t.Setenv("AUDITOR_ALERT_EMAIL_TO", "")

	store := &fakeDigestStore{
		digests:     map[string]*repository.QueryDigest{},
		destsByEnv:  map[string][]repository.NotificationDestination{},
		rawTargets:  map[string]string{},
		seedFPByEnv: map[string]string{envA: fpA, envB: fpB},
		cfg:         &repository.QueryDigestConfig{WindowDays: 7, DeliveryEnabled: true, Thresholds: json.RawMessage(`{}`)},
	}
	idA, idB := "00000000-0000-0000-0000-0000000000a1", "00000000-0000-0000-0000-0000000000b1"
	store.destsByEnv[envA] = []repository.NotificationDestination{{
		ID: idA, EnvironmentID: envA, Channel: "webhook", Format: "generic", Active: true,
		TargetMasked: repository.MaskDestination("webhook", srvA.URL),
	}}
	store.destsByEnv[envB] = []repository.NotificationDestination{{
		ID: idB, EnvironmentID: envB, Channel: "webhook", Format: "generic", Active: true,
		TargetMasked: repository.MaskDestination("webhook", srvB.URL),
	}}
	store.rawTargets[idA], store.rawTargets[idB] = srvA.URL, srvB.URL

	op := fakeAuthStore{user: &repository.AuditorUser{Role: "operator", Environments: []string{envA, envB}}}
	h := NewHandlerWithOptions(store, HandlerOptions{Auth: op})
	for _, env := range []string{envA, envB} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/environments/"+env+"/query-digests", strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer valid-token")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("generate %s: %d", env, w.Code)
		}
	}

	// List A must not include B fingerprints.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/environments/"+envA+"/query-digests", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("list A: %d", w.Code)
	}
	if strings.Contains(w.Body.String(), fpB) || strings.Contains(w.Body.String(), envB) {
		t.Fatalf("list A mixed B data: %s", w.Body.String())
	}

	store.mu.Lock()
	for _, d := range store.deliveries {
		if d.EnvironmentID == envA && d.DestinationID == idB {
			t.Fatal("delivery for A used destination of B")
		}
		if d.EnvironmentID == envB && d.DestinationID == idA {
			t.Fatal("delivery for B used destination of A")
		}
		if strings.Contains(d.Body, fpB) && d.EnvironmentID == envA {
			t.Fatal("A delivery body contained B fingerprint")
		}
	}
	store.mu.Unlock()

	viewer := fakeAuthStore{user: &repository.AuditorUser{Role: "viewer", Environments: []string{envA}}}
	hv := NewHandlerWithOptions(store, HandlerOptions{Auth: viewer})
	req = httptest.NewRequest(http.MethodGet, "/api/v1/environments/"+envB+"/query-digests", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w = httptest.NewRecorder()
	hv.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("viewer without B grant: status %d want 403 body=%s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), fpB) {
		t.Fatal("403 body must not reveal B fingerprints")
	}
}

// TestDigestDeliveryFailureRecordedAndRetried — RNF-05: webhook failure is recorded and retryable.
func TestDigestDeliveryFailureRecordedAndRetried(t *testing.T) {
	env := "00000000-0000-0000-0000-000000000001"
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "SELECT ") || strings.Contains(string(body), "INSERT INTO") {
			t.Errorf("webhook leaked SQL: %s", body)
		}
		if n == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv("AUDITOR_ALERT_WEBHOOK_URL", "")
	store := &fakeDigestStore{
		digests:    map[string]*repository.QueryDigest{},
		destsByEnv: map[string][]repository.NotificationDestination{},
		rawTargets: map[string]string{},
		cfg:        &repository.QueryDigestConfig{WindowDays: 7, DeliveryEnabled: true, Thresholds: json.RawMessage(`{}`)},
	}
	destID := "00000000-0000-0000-0000-000000000077"
	store.destsByEnv[env] = []repository.NotificationDestination{{
		ID: destID, EnvironmentID: env, Channel: "webhook", Format: "generic", Active: true,
		TargetMasked: repository.MaskDestination("webhook", srv.URL),
	}}
	store.rawTargets[destID] = srv.URL

	auth := fakeAuthStore{user: &repository.AuditorUser{Role: "operator", Environments: []string{env}}}
	h := NewHandlerWithOptions(store, HandlerOptions{Auth: auth})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/environments/"+env+"/query-digests", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer valid-token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("status %d body=%s", w.Code, w.Body.String())
	}
	var created repository.QueryDigest
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	store.mu.Lock()
	failed := 0
	for _, d := range store.deliveries {
		if d.DigestID == created.ID && d.Status == "failed" {
			failed++
		}
	}
	claimed := store.claimed[env+"|"+created.ID]
	store.mu.Unlock()
	if failed == 0 {
		t.Fatal("expected failed delivery recorded")
	}
	if claimed {
		t.Fatal("failed delivery must not ClaimAlert / mark as delivered")
	}

	if err := store.RetryFailedQueryDigestDeliveries(context.Background(), env); err != nil {
		t.Fatal(err)
	}
	if calls.Load() < 2 {
		t.Fatalf("retry did not re-hit webhook: calls=%d", calls.Load())
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	sent := false
	for _, d := range store.deliveries {
		if d.DigestID == created.ID && d.Status == "sent" {
			sent = true
		}
	}
	if !sent {
		t.Fatal("retry must record a successful delivery")
	}
	if !store.claimed[env+"|"+created.ID] {
		t.Fatal("successful retry must claim digest delivery")
	}
}

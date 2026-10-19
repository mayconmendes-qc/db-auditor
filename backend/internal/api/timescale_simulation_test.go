package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/osmendes/db-auditor/internal/repository"
	"github.com/osmendes/db-auditor/internal/tsretention"
)

type simStore struct {
	stubStore
	input     *repository.HypertableSimulationInput
	scenarios []repository.RetentionScenario
	price     *repository.CostPriceVersion
	saved     *repository.RetentionScenario
	findings  int
}

func (s *simStore) LoadHypertableSimulationInput(context.Context, string, string, string, string, string) (*repository.HypertableSimulationInput, error) {
	if s.input != nil {
		return s.input, nil
	}
	return &repository.HypertableSimulationInput{NotApplicable: true, Reason: "não aplicável"}, nil
}
func (s *simStore) ListRetentionScenarios(context.Context, string) ([]repository.RetentionScenario, error) {
	return s.scenarios, nil
}
func (s *simStore) SaveRetentionScenario(_ context.Context, env string, in repository.SaveRetentionScenarioInput) (*repository.RetentionScenario, error) {
	item := repository.RetentionScenario{
		ID: "sc-1", EnvironmentID: env, DatabaseName: in.DatabaseName, SchemaName: in.SchemaName,
		HypertableName: in.HypertableName, Label: in.Label, RetentionDays: in.RetentionDays,
		CompressAfterDays: in.CompressAfterDays, GBSaved: in.Scenario.GBSaved, SavedBy: in.SavedBy,
		SavedAt: time.Now().UTC(), Risks: in.Scenario.Risks,
	}
	s.saved = &item
	return &item, nil
}
func (s *simStore) CurrentCostPrice(context.Context, string) (*repository.CostPriceVersion, error) {
	return s.price, nil
}
func (s *simStore) ListChunkScanObservations(context.Context, string, string, string, string, string) ([]tsretention.ChunkScanObservation, error) {
	return nil, nil
}
func (s *simStore) PreviousCompleteRunID(context.Context, string, string) (string, error) {
	return "", nil
}
func (s *simStore) UpsertFinding(context.Context, repository.UpsertFindingParams) (*repository.Finding, error) {
	s.findings++
	return &repository.Finding{ID: "f1"}, nil
}

func TestSimulationNotApplicableCA05(t *testing.T) {
	store := &simStore{}
	h := NewHandlerWithOptions(store, HandlerOptions{})
	env := "11111111-1111-1111-1111-111111111111"
	run := "22222222-2222-2222-2222-222222222222"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/environments/"+env+"/runs/"+run+"/databases/db/schemas/public/hypertables/metrics/simulation", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "não aplicável") {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestInvalidScenario400(t *testing.T) {
	now := time.Now().UTC()
	end := now.Add(-48 * time.Hour)
	store := &simStore{input: &repository.HypertableSimulationInput{
		Found: true,
		Chunks: []repository.SimulationChunkRow{{
			ChunkSchema: "_timescaledb_internal", ChunkName: "c1", RangeEnd: &end, TotalSizeBytes: 1 << 30,
		}},
	}}
	h := NewHandlerWithOptions(store, HandlerOptions{})
	env := "11111111-1111-1111-1111-111111111111"
	run := "22222222-2222-2222-2222-222222222222"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/environments/"+env+"/runs/"+run+"/databases/db/schemas/public/hypertables/metrics/simulation?retention_days=0", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400 body=%s", rec.Code, rec.Body.String())
	}
}

func TestViewerCannotSaveScenarioCA07(t *testing.T) {
	env := "11111111-1111-1111-1111-111111111111"
	auth := fakeAuthStore{user: &repository.AuditorUser{Role: "viewer", Environments: []string{env}}}
	store := &simStore{}
	h := NewHandlerWithOptions(store, HandlerOptions{Auth: auth})
	body := `{"database_name":"db","schema_name":"public","hypertable_name":"metrics","retention_days":90,"audit_run_id":"22222222-2222-2222-2222-222222222222"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/environments/"+env+"/timescale-scenarios", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer valid-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403 body=%s", rec.Code, rec.Body.String())
	}
}

func TestSimulationCurrencyWithPriceCA06(t *testing.T) {
	now := time.Now().UTC()
	end := now.Add(-100 * 24 * time.Hour)
	eff := now.Add(-24 * time.Hour)
	store := &simStore{
		input: &repository.HypertableSimulationInput{
			Found: true,
			Chunks: []repository.SimulationChunkRow{{
				ChunkSchema: "s", ChunkName: "c", RangeEnd: &end, TotalSizeBytes: 10 * 1024 * 1024 * 1024,
			}},
		},
		price: &repository.CostPriceVersion{ID: "pv1", Currency: "BRL", StorageGBMonth: 0.5, EffectiveFrom: eff},
	}
	h := NewHandlerWithOptions(store, HandlerOptions{})
	env := "11111111-1111-1111-1111-111111111111"
	run := "22222222-2222-2222-2222-222222222222"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/environments/"+env+"/runs/"+run+"/databases/db/schemas/public/hypertables/metrics/simulation?retention_days=30", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var res simulationAPIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.Currency != "BRL" || res.PriceVersionID != "pv1" || res.PriceEffectiveFrom == nil {
		t.Fatalf("currency/price missing: %+v", res)
	}
	if len(res.Scenarios) == 0 || res.Scenarios[0].MoneySaved == nil {
		t.Fatalf("money_saved missing: %+v", res.Scenarios)
	}
}

func TestExportScenariosCSVMarker(t *testing.T) {
	store := &simStore{scenarios: []repository.RetentionScenario{{
		DatabaseName: "db", SchemaName: "public", HypertableName: "metrics", Label: "ret-90",
		RetentionDays: 90, GBSaved: 10, SavedAt: time.Now().UTC(),
	}}}
	h := NewHandlerWithOptions(store, HandlerOptions{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/environments/11111111-1111-1111-1111-111111111111/timescale-scenarios/export?format=csv", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "# export_complete") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

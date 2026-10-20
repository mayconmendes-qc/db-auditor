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

	"github.com/osmendes/db-auditor/internal/costlens"
	"github.com/osmendes/db-auditor/internal/repository"
)

type fakeCostStore struct {
	stubStore
	prices    *repository.CostPricesResponse
	created   *repository.CostPriceVersion
	createErr error
	ranking   *repository.CostRankingResponse
	series    []costlens.SeriesPoint
}

func (f *fakeCostStore) ListCostPrices(context.Context, string) (*repository.CostPricesResponse, error) {
	if f.prices == nil {
		return &repository.CostPricesResponse{History: []repository.CostPriceVersion{}}, nil
	}
	return f.prices, nil
}
func (f *fakeCostStore) CreateCostPriceVersion(_ context.Context, _, _ string, in costlens.PriceInput) (*repository.CostPriceVersion, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	v := &repository.CostPriceVersion{
		ID: "p1", EnvironmentID: "00000000-0000-0000-0000-000000000001", Version: 1,
		Currency: costlens.NormalizeCurrency(in.Currency), StorageGBMonth: in.StorageGBMonth,
		EffectiveFrom: in.EffectiveFrom, CreatedBy: "op", CreatedAt: time.Now().UTC(), ModelOrigin: "custom",
	}
	f.created = v
	return v, nil
}
func (f *fakeCostStore) ComputeCostRanking(context.Context, repository.CostRankingFilter) (*repository.CostRankingResponse, error) {
	if f.ranking == nil {
		r := costlens.BuildRanking(nil, 10, nil, "sem coleta completa")
		return &repository.CostRankingResponse{Ranking: r}, nil
	}
	return f.ranking, nil
}
func (f *fakeCostStore) ListCostSeries(context.Context, string, time.Time, time.Time) ([]costlens.SeriesPoint, error) {
	return f.series, nil
}
func (f *fakeCostStore) TableStorageCostMap(context.Context, string) (map[string]costlens.CostResult, error) {
	return map[string]costlens.CostResult{}, nil
}

func TestPriceTableRequiresOperator(t *testing.T) {
	env := "00000000-0000-0000-0000-000000000001"
	store := &fakeCostStore{}
	for _, tc := range []struct {
		role string
		want int
	}{
		{"viewer", 403},
		{"auditor", 403},
		{"operator", 201},
	} {
		auth := fakeAuthStore{user: &repository.AuditorUser{Role: tc.role, Environments: []string{env}}}
		h := NewHandlerWithOptions(store, HandlerOptions{Auth: auth})
		body := `{"currency":"BRL","storage_gb_month":0.5,"effective_from":"2026-01-01T00:00:00Z"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/environments/"+env+"/cost/prices", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer valid-token")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != tc.want {
			t.Fatalf("%s: status %d want %d body=%s", tc.role, w.Code, tc.want, w.Body.String())
		}
	}
}

func TestCostCSVCompleteMarkerNoSQL(t *testing.T) {
	amt := 1.0
	ranking := &repository.CostRankingResponse{Ranking: costlens.Ranking{
		Items: []costlens.CostResult{{
			Type: "unused_index", ObjectKey: "db.public.idx", Database: "db", Schema: "public", Object: "idx",
			SizeBytes: 2e9, MonthAmount: &amt, Currency: "BRL", Priced: true,
			Formula: "2 GB x BRL 0.50", Fingerprint: "abc123",
		}},
	}}
	csv := repository.FormatCostRankingCSV(ranking)
	if !strings.Contains(csv, "# export_complete") {
		t.Fatalf("missing marker: %q", csv)
	}
	lower := strings.ToLower(csv)
	for _, bad := range []string{"select ", " from ", " where ", "insert ", "update "} {
		if strings.Contains(lower, bad) {
			t.Fatalf("CSV contains SQL fragment %q", bad)
		}
	}
}

func TestRankingWithoutPrices(t *testing.T) {
	env := "00000000-0000-0000-0000-000000000001"
	store := &fakeCostStore{ranking: &repository.CostRankingResponse{Ranking: costlens.Ranking{
		Items: []costlens.CostResult{{
			Type: "storage", ObjectKey: "db.public.t", SizeBytes: 1e9,
			Reason: costlens.ReasonNoPrice, Label: costlens.ReasonNoPrice, Priced: false,
		}},
		Priced: false,
	}}}
	auth := fakeAuthStore{user: &repository.AuditorUser{Role: "viewer", Environments: []string{env}}}
	h := NewHandlerWithOptions(store, HandlerOptions{Auth: auth})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/environments/"+env+"/cost/ranking", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
	}
	var res repository.CostRankingResponse
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 1 || res.Items[0].Reason != costlens.ReasonNoPrice {
		t.Fatalf("want not estimated: %+v", res.Items)
	}
}

func TestPriceValidationAPI(t *testing.T) {
	env := "00000000-0000-0000-0000-000000000001"
	store := &fakeCostStore{createErr: costlens.ValidatePriceInput(costlens.PriceInput{Currency: "EUR", StorageGBMonth: 1, EffectiveFrom: time.Now()}, "")}
	auth := fakeAuthStore{user: &repository.AuditorUser{Role: "operator", Environments: []string{env}}}
	h := NewHandlerWithOptions(store, HandlerOptions{Auth: auth})
	body := `{"currency":"EUR","storage_gb_month":1,"effective_from":"2026-01-01T00:00:00Z"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/environments/"+env+"/cost/prices", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer valid-token")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", rr.Code, rr.Body.String())
	}
}

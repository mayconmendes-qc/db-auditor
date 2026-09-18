package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAnalyticsKPIs(t *testing.T) {
	t.Parallel()
	h := NewHandler(&stubStore{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/kpis", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["environments"]; !ok {
		t.Fatalf("missing environments: %v", body)
	}
}

func TestAnalyticsStorage(t *testing.T) {
	t.Parallel()
	h := NewHandler(&stubStore{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/storage", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestReportsInventory(t *testing.T) {
	t.Parallel()
	h := NewHandler(&stubStore{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/inventory", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
}

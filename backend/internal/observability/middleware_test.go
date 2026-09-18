package observability

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMiddlewareSetsRequestID(t *testing.T) {
	t.Parallel()
	h := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if RequestIDFromContext(r.Context()) == "" {
			t.Fatal("missing request id in context")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Header().Get("X-Request-ID") == "" {
		t.Fatal("missing X-Request-ID header")
	}
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestMetricsHandler(t *testing.T) {
	t.Parallel()
	m := NewMetrics()
	m.ObserveHTTP(http.MethodGet, "/api/v1/environments/aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee/tables", 200, 0)
	h := m.Handler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	body := w.Body.String()
	if !strings.Contains(body, "auditor_up 1") {
		t.Fatalf("body missing auditor_up: %s", body)
	}
	if !strings.Contains(body, `path="/api/v1/environments/:id/tables"`) {
		t.Fatalf("expected path cardinality collapse: %s", body)
	}
}

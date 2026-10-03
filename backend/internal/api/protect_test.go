package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPublicMetricsHidden(t *testing.T) {
	t.Parallel()
	h := NewHandler(&stubStore{})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestBodyLimit(t *testing.T) {
	t.Parallel()
	h := NewHandler(&stubStore{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/audit-runs", strings.NewReader("x"))
	req.ContentLength = maxAPIBody + 1
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestMutationRateLimit(t *testing.T) {
	t.Parallel()
	h := protectPublic(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	for i := 0; i < 20; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/audit-runs", nil)
		req.RemoteAddr = "203.0.113.8:1234"
		h.ServeHTTP(w, req)
		if w.Code != http.StatusAccepted {
			t.Fatalf("request %d status %d", i, w.Code)
		}
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/audit-runs", nil)
	req.RemoteAddr = "203.0.113.8:1234"
	h.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d", w.Code)
	}
}

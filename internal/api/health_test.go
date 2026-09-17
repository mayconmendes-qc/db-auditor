package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeStore struct{ err error }

func (s fakeStore) Ping(context.Context) error { return s.err }

func TestHealth(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	NewHandler(fakeStore{}).ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestReady(t *testing.T) {
	tests := []struct {
		name  string
		store fakeStore
		want  int
	}{
		{"database is available", fakeStore{}, http.StatusOK},
		{"database is unavailable", fakeStore{errors.New("down")}, http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			NewHandler(tt.store).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ready", nil))
			if w.Code != tt.want {
				t.Fatalf("status = %d, want %d", w.Code, tt.want)
			}
		})
	}
}

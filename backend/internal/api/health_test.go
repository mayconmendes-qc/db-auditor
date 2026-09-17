package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mayconmendes-qc/timescale-auditor/internal/repository"
)

type stubStore struct{}

func (stubStore) Ping(context.Context) error { return nil }

func (stubStore) ListEnvironmentsAPI(context.Context) ([]repository.Environment, error) {
	return []repository.Environment{}, nil
}

func (stubStore) ListDatabaseSnapshots(context.Context, string) ([]repository.DatabaseSnapshot, error) {
	return []repository.DatabaseSnapshot{}, nil
}

func (stubStore) ListSchemaSnapshots(context.Context, string) ([]repository.SchemaSnapshot, error) {
	return []repository.SchemaSnapshot{}, nil
}

type captureWriter struct {
	code int
	body bytes.Buffer
	hdr  http.Header
}

func (c *captureWriter) Header() http.Header {
	if c.hdr == nil {
		c.hdr = make(http.Header)
	}
	return c.hdr
}

func (c *captureWriter) Write(b []byte) (int, error) {
	if c.code == 0 {
		c.code = http.StatusOK
	}
	return c.body.Write(b)
}

func (c *captureWriter) WriteHeader(statusCode int) {
	c.code = statusCode
}

func TestHealth(t *testing.T) {
	h := NewHandler(stubStore{})
	w := &captureWriter{}
	r, err := http.NewRequest(http.MethodGet, "/health", nil)
	if err != nil {
		t.Fatal(err)
	}
	h.ServeHTTP(w, r)
	if w.code != http.StatusOK {
		t.Fatalf("status %d", w.code)
	}
	var payload map[string]string
	if err := json.Unmarshal(w.body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["status"] != "ok" {
		t.Fatalf("unexpected payload %#v", payload)
	}
}

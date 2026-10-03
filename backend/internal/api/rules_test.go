package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mayconmendes-qc/db-auditor/internal/analyzer"
)

type rulesStoreStub struct{ *stubStore }

func (*rulesStoreStub) EffectiveRules(_ context.Context, env, schema string) ([]analyzer.EffectiveRule, error) {
	return analyzer.EffectiveCatalog(analyzer.SnapshotFacts{EnvironmentID: env}, schema), nil
}

func TestRulesEndpointReturnsVersionedCatalog(t *testing.T) {
	h := NewHandler(&rulesStoreStub{stubStore: &stubStore{}})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/environments/abc/rules?schema=public", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Items  []analyzer.EffectiveRule `json:"items"`
		Schema string                   `json:"schema"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Schema != "public" || len(body.Items) != len(analyzer.Catalog()) {
		t.Fatalf("invalid rules response: schema=%s items=%d", body.Schema, len(body.Items))
	}
	var storage string
	for _, item := range body.Items {
		if item.ID == "storage.large_table" {
			storage = item.Version
		}
	}
	if storage != "1.0.0" {
		t.Fatalf("storage rule version %s", storage)
	}
}

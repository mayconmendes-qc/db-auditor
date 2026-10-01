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
	if body.Schema != "public" || len(body.Items) != len(analyzer.Catalog()) || body.Items[0].Version != "1.0.0" {
		t.Fatalf("invalid rules response: %#v", body)
	}
}

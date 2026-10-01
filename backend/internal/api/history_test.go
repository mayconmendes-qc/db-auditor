package api

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestHistoryFilterDefaultsAndValidation(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/environments/env/history?database=db&schema=public&table=orders", nil)
	req.SetPathValue("id", "env")
	f, ok := historyFilter(req)
	if !ok || f.Granularity != "day" || f.DatabaseName != "db" || f.SchemaName != "public" || f.TableName != "orders" {
		t.Fatalf("unexpected filter: %#v ok=%v", f, ok)
	}
	if !f.From.Before(f.To) || f.To.Sub(f.From) < 29*24*time.Hour {
		t.Fatalf("unexpected default interval: %s to %s", f.From, f.To)
	}
}

func TestHistoryFilterRejectsIncompleteTableScope(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/environments/env/tables/history?database=db", nil)
	req.SetPathValue("id", "env")
	if f, ok := historyFilter(req); !ok || tableScopeComplete(f) {
		t.Fatal("expected incomplete table scope to be rejected")
	}
}

func TestHistoryFilterRejectsInvalidGranularityAndDates(t *testing.T) {
	for _, query := range []string{
		"granularity=quarter", "from=not-a-date", "to=not-a-date",
	} {
		req := httptest.NewRequest("GET", "/api/v1/environments/env/history?"+query, nil)
		req.SetPathValue("id", "env")
		if _, ok := historyFilter(req); ok {
			t.Fatalf("expected invalid filter %q to be rejected", query)
		}
	}
}

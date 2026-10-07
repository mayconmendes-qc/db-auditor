package repository

import (
	"strings"
	"testing"
)

func TestInventoryOrderUsesAllowlistAndStableTieBreaker(t *testing.T) {
	columns := map[string]string{"size": "size_bytes"}
	if got := inventoryOrder("size:desc", columns, "id"); got != "size_bytes DESC NULLS LAST, id ASC" {
		t.Fatalf("unexpected order: %s", got)
	}
	for _, raw := range []string{"size:drop table", "unknown:asc", "size:asc;SELECT 1"} {
		if got := inventoryOrder(raw, columns, "id"); got != "id" {
			t.Fatalf("unsafe order %q returned %q", raw, got)
		}
	}
	query := orderedInventorySQL(listIndexesSQL, "size:desc", columns)
	if !strings.Contains(query, "ORDER BY size_bytes DESC NULLS LAST, id ASC\nLIMIT") {
		t.Fatalf("sort was not applied before pagination: %s", query)
	}
}

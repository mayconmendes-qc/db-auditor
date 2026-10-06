package actions

import "testing"

func TestActionCatalogCoversMajorDatabaseDecisions(t *testing.T) {
	cases := []struct{ rule, category string }{
		{"model.no_primary_key", "structure"},
		{"index.invalid", "structure"},
		{"performance.sequential_scan", "query_and_index"},
		{"vacuum.dead_tuples", "maintenance"},
		{"security.excessive_privilege", "security"},
		{"unknown.rule", "investigation"},
	}
	for _, tc := range cases {
		plan := For(tc.rule)
		if plan.Category != tc.category || plan.Prerequisites == "" || plan.Confirmation == "" || plan.Validation == "" || plan.Risk == "" || plan.FalsePositive == "" {
			t.Fatalf("rule %s has incomplete plan: %+v", tc.rule, plan)
		}
	}
	if For("model.no_primary_key").ReadOnlyQuery == "" {
		t.Fatal("primary key plan needs a read-only verification query")
	}
}

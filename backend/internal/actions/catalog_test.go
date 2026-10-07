package actions

import "testing"

func TestActionCatalogCoversMajorDatabaseDecisions(t *testing.T) {
	cases := []struct{ rule, category string }{
		{"integrity.missing_primary_key", "structure"},
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
	if For("integrity.missing_primary_key").ReadOnlyQuery == "" {
		t.Fatal("primary key plan needs a read-only verification query")
	}
	for _, rule := range []string{"integrity.constraint_unvalidated", "index.invalid", "integrity.orphan_sequence", "model.naming_inconsistent"} {
		if For(rule).ReadOnlyQuery == "" {
			t.Fatalf("structural rule %s needs a confirmation query", rule)
		}
	}
}

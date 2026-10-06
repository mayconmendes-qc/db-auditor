package capabilities

import "testing"

func TestRulesAreNotAppliedToUnsupportedEngines(t *testing.T) {
	if RuleApplicable("mysql", "security.excessive_privilege", false) {
		t.Fatal("PostgreSQL rule applied to unsupported engine")
	}
	if RuleApplicable("postgresql", "chunk.high_count", false) {
		t.Fatal("Timescale rule applied without capability")
	}
	if !RuleApplicable("postgresql", "chunk.high_count", true) {
		t.Fatal("observed Timescale capability ignored")
	}
	for _, item := range Matrix("unknown_engine", false, false) {
		if item.Applicable || item.Reason == "" {
			t.Fatalf("unsupported engine did not explain non-applicability: %+v", item)
		}
	}
}

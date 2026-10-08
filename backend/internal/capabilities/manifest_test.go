package capabilities

import "testing"

func TestRulesAreNotAppliedToUnsupportedEngines(t *testing.T) {
	if RuleApplicable("mongodb", "integrity.missing_primary_key", false) {
		t.Fatal("PostgreSQL rule applied to MongoDB")
	}
	mongo := Matrix("mongodb", false, false)
	for _, item := range mongo {
		if item.Applicable != (item.Name == "catalog" || item.Name == "indexes") {
			t.Fatalf("incorrect MongoDB capability: %+v", item)
		}
	}
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

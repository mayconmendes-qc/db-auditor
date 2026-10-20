package dqcontract

import "testing"

func TestYAMLErrorsLineAndField(t *testing.T) {
	_, _, err := ParseYAML([]byte("database: app\nschema: public\ntable: orders\nsample_limit: -1\nchecks: []\n"), DefaultSampleLimit)
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(*YAMLError); !ok {
		t.Fatalf("want YAMLError, got %T %v", err, err)
	}
}

func TestParseValidYAML(t *testing.T) {
	raw := []byte("database: app\nschema: public\ntable: orders\nsample_limit: 10000\nchecks:\n  - type: nulls\n    column: id\n    max_null_pct: 0\n")
	doc, norm, err := ParseYAML(raw, DefaultSampleLimit)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Table != "orders" || len(doc.Checks) != 1 || norm == "" {
		t.Fatalf("unexpected: %+v", doc)
	}
}

func TestSampleLimitAboveProcessCeiling(t *testing.T) {
	zero := 0.0
	doc := Document{Database: "d", Schema: "s", Table: "t", SampleLimit: 50000, Checks: []Check{{Type: CheckNulls, Column: "id", MaxNullPct: &zero}}}
	if err := ValidateDocument(doc, 10000); err == nil {
		t.Fatal("expected sample_limit rejection")
	}
}

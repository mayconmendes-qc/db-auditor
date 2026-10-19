package dqcontract

import "testing"

func TestProposalFromSnapshot(t *testing.T) {
	doc := ProposeFromSnapshot(ProposalInput{
		Database: "app", Schema: "public", Table: "orders",
		Columns: []SnapshotColumn{
			{Name: "id", DataType: "bigint", IsNullable: false},
			{Name: "customer_id", DataType: "bigint", IsNullable: true},
			{Name: "updated_at", DataType: "timestamptz", IsNullable: true},
		},
		Constraints: []SnapshotConstraint{
			{Type: "p", ConstrainedColumns: []string{"id"}},
			{Type: "f", ConstrainedColumns: []string{"customer_id"}, ReferencedSchema: "public", ReferencedTable: "customers", ReferencedColumns: []string{"id"}},
		},
	})
	if doc.SampleLimit != DefaultSampleLimit {
		t.Fatalf("sample_limit: %d", doc.SampleLimit)
	}
	var hasNulls, hasUnique, hasOrphan, hasFresh bool
	for _, c := range doc.Checks {
		switch c.Type {
		case CheckNulls:
			if c.Column == "id" && c.MaxNullPct != nil && *c.MaxNullPct == 0 {
				hasNulls = true
			}
		case CheckUniqueness:
			hasUnique = true
		case CheckOrphan:
			hasOrphan = true
		case CheckFreshness:
			hasFresh = true
		}
	}
	if !hasNulls || !hasUnique || !hasOrphan || !hasFresh {
		t.Fatalf("expected nulls/unique/orphan/freshness, got %+v", doc.Checks)
	}
}

package postgres

import "testing"

func TestClassifyRelation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		relkind     string
		isPartition bool
		want        string
	}{
		{"r", false, RelationClassTable},
		{"p", false, RelationClassPartitionedTable},
		{"f", false, RelationClassForeignTable},
		{"r", true, RelationClassPartition},
		{"p", true, RelationClassPartition},
		{"v", false, RelationClassUnknown},
		{"", false, RelationClassUnknown},
	}
	for _, tc := range cases {
		got := ClassifyRelation(tc.relkind, tc.isPartition)
		if got != tc.want {
			t.Fatalf("ClassifyRelation(%q,%v)=%q want %q", tc.relkind, tc.isPartition, got, tc.want)
		}
	}
}

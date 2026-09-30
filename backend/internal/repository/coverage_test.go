package repository

import "testing"

func TestClassifySnapshotCompleteness(t *testing.T) {
	t.Parallel()
	cases := []struct {
		status  string
		failDB  int
		failCol int
		want    string
	}{
		{"success", 0, 0, "complete"},
		{"SUCCESS", 0, 0, "complete"},
		{"partial_success", 0, 0, "partial"},
		{"success", 1, 0, "partial"},
		{"success", 0, 2, "partial"},
		{"failed", 0, 0, "empty"},
		{"running", 0, 0, "unknown"},
		{"cancelled", 0, 0, "unknown"},
	}
	for _, tc := range cases {
		got := ClassifySnapshotCompleteness(tc.status, tc.failDB, tc.failCol)
		if got != tc.want {
			t.Fatalf("ClassifySnapshotCompleteness(%q,%d,%d)=%q want %q",
				tc.status, tc.failDB, tc.failCol, got, tc.want)
		}
	}
}

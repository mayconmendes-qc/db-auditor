package report

import (
	"testing"
	"time"
)

func TestInsertionClassificationAndAggregation(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC)
	cutoff := time.Date(2025, 9, 30, 12, 0, 0, 0, time.UTC)
	before := cutoff.Add(-time.Nanosecond)
	for _, tc := range []struct {
		name string
		e    *InsertionEvidence
		want InsertionState
	}{
		{"at boundary", &InsertionEvidence{Status: "observed", Latest: &cutoff, AppendOnly: true}, InsertionRecent},
		{"older", &InsertionEvidence{Status: "observed", Latest: &before, AppendOnly: true}, InsertionOld},
		{"deletions", &InsertionEvidence{Status: "observed", Latest: &cutoff}, InsertionIndeterminate},
		{"empty", &InsertionEvidence{Status: "empty", AppendOnly: true}, InsertionIndeterminate},
		{"budget", &InsertionEvidence{Status: "skipped_budget", AppendOnly: true}, InsertionIndeterminate},
		{"missing", nil, InsertionIndeterminate},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyInsertion(asOf, tc.e); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
	for _, tc := range []struct {
		states []InsertionState
		want   InsertionState
	}{
		{nil, InsertionNoTables},
		{[]InsertionState{InsertionOld, InsertionOld}, InsertionOld},
		{[]InsertionState{InsertionOld, InsertionIndeterminate}, InsertionIndeterminate},
		{[]InsertionState{InsertionIndeterminate, InsertionRecent}, InsertionRecent},
	} {
		if got := AggregateInsertion(tc.states); got != tc.want {
			t.Fatalf("aggregate %v: got %q, want %q", tc.states, got, tc.want)
		}
	}
}

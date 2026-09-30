package api

import "testing"

func TestRunStatusClassification(t *testing.T) {
	cases := []struct {
		status     string
		successful bool
		failed     bool
	}{
		{"success", true, false},
		{"SUCCESS", true, false},
		{"partial_success", true, false},
		{"PARTIAL_SUCCESS", true, false},
		{"failed", false, true},
		{"FAILED", false, true},
		{"running", false, false},
		{"cancelled", false, false},
	}
	for _, tc := range cases {
		if got := isSuccessfulRunStatus(tc.status); got != tc.successful {
			t.Fatalf("isSuccessfulRunStatus(%q)=%v want %v", tc.status, got, tc.successful)
		}
		if got := isFailedRunStatus(tc.status); got != tc.failed {
			t.Fatalf("isFailedRunStatus(%q)=%v want %v", tc.status, got, tc.failed)
		}
	}
}

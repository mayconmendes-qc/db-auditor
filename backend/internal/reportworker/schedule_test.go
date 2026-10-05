package reportworker

import "testing"

func TestWeeklyExecutiveDue(t *testing.T) {
	if WeeklyExecutiveDue("daily", "partial_success", "success", false) {
		t.Fatal("partial run must not schedule a PDF")
	}
	if WeeklyExecutiveDue("fast", "success", "success", false) {
		t.Fatal("fast profile is not an executive source")
	}
	if !WeeklyExecutiveDue("weekly", "success", "success", false) {
		t.Fatal("weekly success should enqueue")
	}
	if WeeklyExecutiveDue("weekly", "success", "success", true) {
		t.Fatal("existing report must not duplicate")
	}
}

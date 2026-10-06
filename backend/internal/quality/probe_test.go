package quality

import (
	"testing"
)

func TestRequestValidationBoundsAndDates(t *testing.T) {
	valid := Request{Database: "db", Schema: "public", Table: "orders", Limit: 1000,
		ExpectedNonNull: []string{"customer_id"}, DateRanges: []DateRange{{Column: "created_at", From: "2025-01-01T00:00:00Z", To: "2026-01-01T00:00:00Z"}}}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	invalid := []Request{
		{Database: "db", Schema: "public", Table: "orders", Limit: 1001},
		{Database: "db", Schema: "public", Table: "orders", Limit: 1, ExpectedNonNull: []string{""}},
		{Database: "db", Schema: "public", Table: "orders", Limit: 1, DateRanges: []DateRange{{Column: "created_at", From: "2026-01-01T00:00:00Z", To: "2025-01-01T00:00:00Z"}}},
	}
	for _, item := range invalid {
		if err := item.Validate(); err == nil {
			t.Fatalf("accepted invalid request: %+v", item)
		}
	}
}

func TestSanitationPlansRequireExternalReview(t *testing.T) {
	for _, kind := range []string{"null", "duplicate", "orphan", "date_range", "distribution"} {
		plan := PlanFor(kind)
		if plan.Meaning == "" || plan.Confirmation == "" || plan.ExternalSteps == "" || plan.Validation == "" || plan.Risk == "" {
			t.Fatalf("incomplete %s plan: %+v", kind, plan)
		}
	}
}

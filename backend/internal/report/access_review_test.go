package report

import (
	"strings"
	"testing"
)

func TestAccessReviewPDFIdentifierRedaction(t *testing.T) {
	t.Parallel()
	d := Document{
		Type: "access_review", Environment: "erp-prod", RunID: "run-1",
		AccessReview: &AccessReviewSummary{
			CampaignID: "camp-1", CampaignName: "Q1", Status: "open", DueAt: "2026-12-01",
			TotalItems: 100, DecidedItems: 40, PendingItems: 60, DecidedPercent: 40,
			Owners: []AccessReviewOwnerLine{{TargetRole: "app_report", OwnerName: "alice", Contact: "alice@example.com"}},
			PendingSamples: []AccessReviewPendingLine{{Account: "app_etl", Object: "vendas.pedido", Privilege: "UPDATE", Origin: "direto"}},
		},
	}
	d.RedactSensitive("identifiers")
	if d.AccessReview.Owners[0].OwnerName != redactionMask || d.AccessReview.Owners[0].Contact != "" {
		t.Fatalf("owner not redacted: %+v", d.AccessReview.Owners[0])
	}
	if d.AccessReview.PendingSamples[0].Account != redactionMask {
		t.Fatalf("account not redacted: %+v", d.AccessReview.PendingSamples[0])
	}
	// Counts remain (CA-07).
	if d.AccessReview.TotalItems != 100 || d.AccessReview.PendingItems != 60 {
		t.Fatalf("counts should remain: %+v", d.AccessReview)
	}
	text := ""
	for _, line := range accessReviewLines(d) {
		text += line.Text + "\n"
	}
	if strings.Contains(text, "alice") || strings.Contains(text, "app_etl") || strings.Contains(text, "example.com") {
		t.Fatalf("redacted PDF lines still expose identifiers:\n%s", text)
	}
	if !strings.Contains(text, "100") || !strings.Contains(text, "60") {
		t.Fatalf("counts missing from lines:\n%s", text)
	}
}

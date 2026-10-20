package analyzer

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestAccessReviewRevocationNotApplied(t *testing.T) {
	t.Parallel()
	due := time.Now().UTC().Add(-24 * time.Hour)
	facts := SnapshotFacts{
		EnvironmentID: "env", AuditRunID: "run",
		AccessReviewCampaigns: []AccessReviewCampaignFact{{
			ID: "camp", EnvironmentID: "env", Status: "open", DueAt: time.Now().UTC().Add(24 * time.Hour),
			TotalItems: 1, PendingItems: 0,
			Items: []AccessReviewItemFact{{
				ID: "item", AccountName: "app_etl", ObjectType: "table", ObjectName: "vendas.pedido",
				Privilege: "UPDATE", Decision: "revoke", RevokeDueAt: &due, PrivilegeStillPresent: true,
				FindingActionID: "action-1",
			}},
		}},
	}
	out, err := AccessReviewAnalyzer{}.Analyze(context.Background(), facts)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range out {
		if f.FindingType == "access_review.revocation_not_applied" && f.Severity == SeverityHigh {
			found = true
			if f.Evidence["finding_action_id"] != "action-1" {
				t.Fatalf("expected action link: %+v", f.Evidence)
			}
		}
	}
	if !found {
		t.Fatalf("missing revocation_not_applied: %+v", out)
	}
}

func TestAccessReviewKeepRules(t *testing.T) {
	t.Parallel()
	facts := SnapshotFacts{
		AccessReviewCampaigns: []AccessReviewCampaignFact{{
			ID: "c", Status: "open", DueAt: time.Now().UTC().Add(48 * time.Hour),
			TotalItems: 2, PendingItems: 1,
			Items: []AccessReviewItemFact{
				{ID: "1", AccountName: "su", ObjectName: "x.t", Privilege: "SELECT", Highlights: []string{"powerful_role"}, Decision: ""},
				{ID: "2", AccountName: "r", ObjectName: "vendas.t", Privilege: "SELECT", OriginKind: "public", Decision: "approve"},
			},
		}},
	}
	out, err := AccessReviewAnalyzer{}.Analyze(context.Background(), facts)
	if err != nil {
		t.Fatal(err)
	}
	types := map[string]bool{}
	for _, f := range out {
		types[f.FindingType] = true
	}
	if !types["access_review.powerful_without_approve"] || !types["access_review.public_outside_public"] {
		t.Fatalf("types=%v", types)
	}
}

func TestAccessReviewInactiveCitesLoginSource(t *testing.T) {
	t.Parallel()
	ts := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	facts := SnapshotFacts{
		EnvironmentID: "env", AuditRunID: "run",
		AccessReviewCampaigns: []AccessReviewCampaignFact{{
			ID: "c", EnvironmentID: "env", Status: "open", DueAt: time.Now().UTC().Add(48 * time.Hour),
			TotalItems: 1, PendingItems: 1,
			Items: []AccessReviewItemFact{{
				ID: "i", AccountName: "app_old", ObjectType: "table", ObjectName: "vendas.t",
				Privilege: "UPDATE", Highlights: []string{"inactive_or_expired"},
				LastLoginAt: &ts, LastLoginSource: "log",
			}},
		}},
	}
	out, err := AccessReviewAnalyzer{}.Analyze(context.Background(), facts)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range out {
		if f.FindingType != "access_review.inactive_with_write" {
			continue
		}
		found = true
		if f.Evidence["last_login_source"] != "log" {
			t.Fatalf("evidence=%v", f.Evidence)
		}
		if f.Evidence["session_sample"] != "no_session_observed_or_expired" {
			t.Fatalf("session_sample=%v", f.Evidence["session_sample"])
		}
		if !strings.Contains(f.Summary, "log") {
			t.Fatalf("summary=%s", f.Summary)
		}
	}
	if !found {
		t.Fatalf("missing inactive_with_write: %+v", out)
	}
}

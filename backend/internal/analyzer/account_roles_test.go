package analyzer

import (
	"context"
	"testing"
	"time"
)

func TestAccountInactivityRequiresEvidence(t *testing.T) {
	collected := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	expired := collected.Add(-time.Hour)
	facts := SnapshotFacts{Roles: []RoleFact{
		{Database: "db", RoleName: "expired", Login: true, ValidUntil: &expired, CollectedAt: collected},
		{Database: "db", RoleName: "sampled", Login: true, PossiblyInactive: true, CollectedAt: collected},
		{Database: "db", RoleName: "unknown", Login: true, CollectedAt: collected},
		{Database: "db", RoleName: "active", Login: true, SampledActive: true, CollectedAt: collected},
	}}
	findings, err := (SecurityAnalyzer{}).Analyze(context.Background(), facts)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, finding := range findings {
		if finding.FindingType == "security.account_inactive_review" {
			count++
			if finding.ObjectName != "expired" && finding.ObjectName != "sampled" {
				t.Fatalf("unsupported inactivity inference: %+v", finding)
			}
		}
	}
	if count != 2 {
		t.Fatalf("expected expired and sampled candidates, got %d", count)
	}
}

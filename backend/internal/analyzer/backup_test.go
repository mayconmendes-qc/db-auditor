package analyzer

import (
	"testing"
	"time"
)

func TestBackupDrillOverdue(t *testing.T) {
	old := time.Now().UTC().Add(-31 * 24 * time.Hour)
	out, err := BackupAnalyzer{}.Analyze(t.Context(), SnapshotFacts{
		EnvironmentID:    "env",
		RestorePolicy:    &RestorePolicyFact{RTOTargetSeconds: 600, MaxAgeDays: 30},
		LastRestoreDrill: &RestoreDrillFact{ID: "d1", Succeeded: true, PerformedAt: old},
	})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range out {
		if f.FindingType == "backup.drill_overdue" && f.Severity == SeverityMedium {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected drill_overdue, got %#v", out)
	}
}

func TestBackupRTOExceeded(t *testing.T) {
	rto := 840
	out, err := BackupAnalyzer{}.Analyze(t.Context(), SnapshotFacts{
		EnvironmentID: "env",
		RestorePolicy: &RestorePolicyFact{RTOTargetSeconds: 600, MaxAgeDays: 30},
		LastRestoreDrill: &RestoreDrillFact{
			ID: "d1", Succeeded: true, RTOSeconds: &rto, PerformedAt: time.Now().UTC(),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range out {
		if f.FindingType == "backup.rto_exceeded" && f.Severity == SeverityHigh {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected rto_exceeded, got %#v", out)
	}
}

func TestBackupNoOverdueWhenFresh(t *testing.T) {
	rto := 120
	out, err := BackupAnalyzer{}.Analyze(t.Context(), SnapshotFacts{
		EnvironmentID: "env",
		RestorePolicy: &RestorePolicyFact{RTOTargetSeconds: 600, MaxAgeDays: 30},
		LastRestoreDrill: &RestoreDrillFact{
			ID: "d1", Succeeded: true, RTOSeconds: &rto, PerformedAt: time.Now().UTC(),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range out {
		if f.FindingType == "backup.drill_overdue" || f.FindingType == "backup.rto_exceeded" {
			t.Fatalf("unexpected %s", f.FindingType)
		}
	}
}

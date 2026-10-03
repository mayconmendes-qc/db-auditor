package analyzer

import (
	"context"
	"testing"
)

func TestDriftAndCompareSnapshots(t *testing.T) {
	t.Parallel()
	facts := SnapshotFacts{
		EnvironmentID: "left", AuditRunID: "run-left",
		ServerVersion: "16.4", TimescaleVersion: "2.17.2",
		Settings:   map[string]string{"shared_buffers": "1GB", "work_mem": "4MB", "max_connections": "100"},
		Extensions: []ExtensionFact{{Name: "timescaledb", Version: "2.17.2"}},
		Peers: []ServerSide{{
			EnvironmentID: "right", Name: "dc", AuditRunID: "run-right",
			ServerVersion: "16.4", TimescaleVersion: "2.15.0",
			Settings:   map[string]string{"shared_buffers": "128MB", "work_mem": "4MB", "max_connections": "100"},
			Extensions: []ExtensionFact{{Name: "timescaledb", Version: "2.15.0"}},
		}},
	}
	got, err := DriftAnalyzer{}.Analyze(context.Background(), facts)
	if err != nil {
		t.Fatal(err)
	}
	if !hasType(got, "config.version_drift") || !hasType(got, "config.guc_drift") {
		t.Fatalf("%#v", typesOf(got))
	}
	for _, item := range got {
		if item.FindingType == "config.guc_drift" && item.Severity != SeverityInfo {
			t.Fatalf("shared_buffers must stay informational: %#v", item)
		}
	}
	report := CompareServers(factsToSide(facts), facts.Peers[0])
	if len(report.Rows) == 0 {
		t.Fatal("expected snapshot differences")
	}
}

func TestGUCToleranceSkipsSmallNumericDrift(t *testing.T) {
	t.Parallel()
	facts := SnapshotFacts{
		EnvironmentID: "left",
		Settings:      map[string]string{"max_connections": "100"},
		RulePolicies:  []RulePolicy{{EnvironmentID: "left", RuleID: "config.guc_drift", Enabled: true, Parameters: map[string]any{"numeric_tolerance": 0.1}}},
		Peers: []ServerSide{{
			EnvironmentID: "right", Name: "dc",
			Settings: map[string]string{"max_connections": "105"},
		}},
	}
	got, _ := DriftAnalyzer{}.Analyze(context.Background(), facts)
	if hasType(got, "config.guc_drift") {
		t.Fatalf("tolerance should hide 5%% drift: %#v", typesOf(got))
	}
}

func factsToSide(facts SnapshotFacts) ServerSide {
	return ServerSide{
		EnvironmentID: facts.EnvironmentID, AuditRunID: facts.AuditRunID,
		ServerVersion: facts.ServerVersion, TimescaleVersion: facts.TimescaleVersion,
		Extensions: facts.Extensions, Settings: facts.Settings,
	}
}

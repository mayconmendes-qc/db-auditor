package analyzer

import (
	"context"
	"testing"
)

func TestSchemaDriftAnalyzerOutOfPipeline(t *testing.T) {
	a := SchemaDriftAnalyzer{}
	facts := SnapshotFacts{
		EnvironmentID: "env",
		AuditRunID:    "run",
		IsProduction:  true,
		SchemaChanges: []SchemaChangeFact{{
			ObjectType: "index", ObjectKey: "db.public.idx", ChangeKind: "added", Correlation: "none",
		}},
	}
	got, err := a.Analyze(context.Background(), facts)
	if err != nil || len(got) != 1 {
		t.Fatalf("got=%d err=%v", len(got), err)
	}
	if got[0].FindingType != "drift.out_of_pipeline" || got[0].Severity != SeverityMedium {
		t.Fatalf("%+v", got[0])
	}
}

func TestSchemaDriftAnalyzerSkipsCorrelated(t *testing.T) {
	a := SchemaDriftAnalyzer{}
	facts := SnapshotFacts{
		SchemaChanges: []SchemaChangeFact{{
			ObjectType: "index", ObjectKey: "k", ChangeKind: "added", Correlation: "deployment",
		}},
	}
	got, err := a.Analyze(context.Background(), facts)
	if err != nil || len(got) != 0 {
		t.Fatalf("got=%d err=%v", len(got), err)
	}
}

func TestSchemaDriftDowngradesWithoutProduction(t *testing.T) {
	a := SchemaDriftAnalyzer{}
	facts := SnapshotFacts{
		EnvironmentID: "env", AuditRunID: "run", IsProduction: false,
		SchemaChanges: []SchemaChangeFact{{
			ObjectType: "table", ObjectKey: "t", ChangeKind: "removed", Correlation: "none",
		}},
	}
	got, _ := a.Analyze(context.Background(), facts)
	if len(got) != 1 || got[0].Severity != SeverityMedium { // high -> medium
		t.Fatalf("%+v", got)
	}
}

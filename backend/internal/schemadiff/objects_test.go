package schemadiff

import (
	"strings"
	"testing"

	"github.com/osmendes/db-auditor/internal/comparison"
)

func TestDiffDetectsFunctionBodyChangeByFingerprintOnly(t *testing.T) {
	bodyA := "SELECT 'secret-literal-email@example.com'"
	bodyB := "SELECT 'other'"
	fpA := FingerprintValue(bodyA)
	fpB := FingerprintValue(bodyB)
	if fpA == "" || fpA == fpB {
		t.Fatalf("expected distinct non-empty fingerprints")
	}
	if strings.Contains(fpA, "secret") || strings.Contains(fpA, "@") {
		t.Fatalf("fingerprint leaked literal: %s", fpA)
	}

	source := SnapshotObjects{Functions: []FunctionObj{{
		Database: "db", Schema: "public", Name: "f", IdentityArgs: "",
		DefinitionFingerprint: fpA, Language: "sql",
	}}}
	target := SnapshotObjects{Functions: []FunctionObj{{
		Database: "db", Schema: "public", Name: "f", IdentityArgs: "",
		DefinitionFingerprint: fpB, Language: "sql",
	}}}
	res := Diff(source, target, nil)
	if res.Summary.Drift != 1 {
		t.Fatalf("expected 1 drift, got %+v", res.Summary)
	}
	d := res.Objects[0]
	if d.Status != comparison.StatusDrift {
		t.Fatalf("status=%s", d.Status)
	}
	if d.SourceFP != fpA || d.TargetFP != fpB {
		t.Fatalf("fingerprints not exposed: %+v", d)
	}
	raw := d.SourceFP + d.TargetFP + d.SourceName + d.TargetName
	for _, bad := range []string{"secret-literal", "@example", "SELECT"} {
		if strings.Contains(raw, bad) {
			t.Fatalf("raw definition leaked via %q in %+v", bad, d)
		}
	}
}

func TestDiffUsesObjectMapping(t *testing.T) {
	source := SnapshotObjects{Tables: []TableObj{{
		Database: "db", Schema: "public", Name: "orders_old", Relkind: "r", ColumnCount: 3,
	}}}
	target := SnapshotObjects{Tables: []TableObj{{
		Database: "db", Schema: "public", Name: "orders", Relkind: "r", ColumnCount: 3,
	}}}
	srcKey := Qualify("db", "public", "orders_old")
	tgtKey := Qualify("db", "public", "orders")
	res := Diff(source, target, []NameMapping{{ObjectType: "table", SourceKey: srcKey, TargetKey: tgtKey}})
	if res.Summary.Match != 1 {
		t.Fatalf("expected mapped match, got %+v objects=%+v", res.Summary, res.Objects)
	}
}

func TestDiffOnlySourceAndOnlyTarget(t *testing.T) {
	source := SnapshotObjects{Tables: []TableObj{{
		Database: "db", Schema: "public", Name: "legacy", Relkind: "r",
	}}}
	target := SnapshotObjects{Tables: []TableObj{{
		Database: "db", Schema: "public", Name: "new_tbl", Relkind: "r",
	}}}
	res := Diff(source, target, nil)
	if res.Summary.OnlySource != 1 || res.Summary.OnlyTarget != 1 {
		t.Fatalf("unexpected summary %+v", res.Summary)
	}
}

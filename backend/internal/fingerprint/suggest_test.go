package fingerprint

import "testing"

func TestSuggestDatabaseToSchema(t *testing.T) {
	t.Parallel()
	source := []ObjectRef{{
		Database: "billing", ObjectType: string(ObjectTypeDatabase), ObjectName: "billing",
	}}
	target := []ObjectRef{{
		Database: "tsdb", Schema: "billing", ObjectType: string(ObjectTypeSchema), ObjectName: "billing",
	}}
	cands := SuggestMappings(source, target, "tsdb")
	if len(cands) != 1 {
		t.Fatalf("cands=%d", len(cands))
	}
	if cands[0].RelationType != "database_to_schema" || cands[0].Confidence < 0.8 {
		t.Fatalf("%+v", cands[0])
	}
}

func TestSuggestByFingerprint(t *testing.T) {
	t.Parallel()
	source := []ObjectRef{{
		Database: "db", Schema: "public", ObjectType: string(ObjectTypeTable), ObjectName: "t1", Fingerprint: "abc",
	}}
	target := []ObjectRef{{
		Database: "tsdb", Schema: "public", ObjectType: string(ObjectTypeTable), ObjectName: "t1", Fingerprint: "abc",
	}}
	cands := SuggestMappings(source, target, "tsdb")
	if len(cands) != 1 || cands[0].RelationType != "identical" {
		t.Fatalf("%+v", cands)
	}
}

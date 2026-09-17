package comparison

import "testing"

func TestCompareMatchAndDrift(t *testing.T) {
	t.Parallel()
	source := []ObjectItem{
		{ObjectType: "table", Key: "public.orders", Name: "orders", Fingerprint: "aaa"},
		{ObjectType: "table", Key: "public.only_src", Name: "only_src", Fingerprint: "x"},
	}
	target := []ObjectItem{
		{ObjectType: "table", Key: "public.orders", Name: "orders", Fingerprint: "bbb"},
		{ObjectType: "table", Key: "public.only_tgt", Name: "only_tgt", Fingerprint: "y"},
	}
	res := CompareSets(source, target)
	if res.Summary.Drift != 1 || res.Summary.OnlySource != 1 || res.Summary.OnlyTarget != 1 {
		t.Fatalf("summary=%+v", res.Summary)
	}
}

func TestCompareFieldDiff(t *testing.T) {
	t.Parallel()
	source := []ObjectItem{{
		ObjectType: "table", Key: "public.t", Name: "t",
		Fields: map[string]string{"row_estimate": "10", "has_pk": "true"},
	}}
	target := []ObjectItem{{
		ObjectType: "table", Key: "public.t", Name: "t",
		Fields: map[string]string{"row_estimate": "20", "has_pk": "true"},
	}}
	res := CompareSets(source, target)
	if res.Summary.Drift != 1 {
		t.Fatalf("expected drift, got %+v", res.Summary)
	}
	if len(res.Objects[0].FieldDiffs) != 1 || res.Objects[0].FieldDiffs[0].Field != "row_estimate" {
		t.Fatalf("fields=%+v", res.Objects[0].FieldDiffs)
	}
}

func TestFilterByStatus(t *testing.T) {
	t.Parallel()
	objs := []ObjectDiff{
		{Status: StatusMatch}, {Status: StatusDrift}, {Status: StatusOnlySource},
	}
	got := FilterByStatus(objs, []string{"MATCH", "drift"})
	if len(got) != 2 {
		t.Fatalf("got %d", len(got))
	}
}

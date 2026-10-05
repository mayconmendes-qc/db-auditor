package migrate

import "testing"

func TestDecide(t *testing.T) {
	t.Parallel()
	sum := "abc"
	applied := map[string]string{"01_baseline.sql": sum}
	action, err := Decide("01_baseline.sql", sum, applied, true)
	if err != nil || action != Skip {
		t.Fatalf("applied baseline: %v %v", action, err)
	}
	if _, err = Decide("01_baseline.sql", "other", applied, true); err == nil {
		t.Fatal("checksum drift must refuse")
	}
	action, err = Decide("01_baseline.sql", sum, map[string]string{}, true)
	if err != nil || action != Stamp {
		t.Fatalf("existing schema: %v %v", action, err)
	}
	action, err = Decide("01_baseline.sql", sum, map[string]string{}, false)
	if err != nil || action != Execute {
		t.Fatalf("empty schema: %v %v", action, err)
	}
	action, err = Decide("03_audit_schedule.sql", sum, map[string]string{}, true)
	if err != nil || action != Execute {
		t.Fatalf("incremental: %v %v", action, err)
	}
}

func TestListSQLSkipsSeed(t *testing.T) {
	t.Parallel()
	names, err := listSQL("../../migrations")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) < 2 || names[0] != "01_baseline.sql" {
		t.Fatalf("names=%v", names)
	}
	for _, name := range names {
		if name == "02_seed_demo.sql" {
			t.Fatal("seed must not be applied by the API")
		}
	}
}

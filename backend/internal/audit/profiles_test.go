package audit

import "testing"

func TestLiveCollectorProfilesDifferentiation(t *testing.T) {
	t.Parallel()
	// fast: topology only
	fast := liveCollectorProfiles("postgres.server")
	if _, ok := fast[ProfileFast]; !ok {
		t.Fatal("postgres.server must be in fast")
	}
	fastDB := liveCollectorProfiles("postgres.databases")
	if _, ok := fastDB[ProfileFast]; !ok {
		t.Fatal("postgres.databases must be in fast")
	}
	tables := liveCollectorProfiles("postgres.tables")
	if _, ok := tables[ProfileFast]; ok {
		t.Fatal("postgres.tables must not be in fast")
	}
	if _, ok := tables[ProfileDaily]; !ok {
		t.Fatal("postgres.tables must be in daily")
	}
	if _, ok := tables[ProfileWeekly]; !ok {
		t.Fatal("postgres.tables must be in weekly")
	}
	// columns: weekly+monthly+manual, not daily/fast
	cols := liveCollectorProfiles("postgres.columns")
	if _, ok := cols[ProfileDaily]; ok {
		t.Fatal("postgres.columns must not be in daily")
	}
	if _, ok := cols[ProfileWeekly]; !ok {
		t.Fatal("postgres.columns must be in weekly")
	}
	if _, ok := cols[ProfileMonthly]; !ok {
		t.Fatal("postgres.columns must be in monthly")
	}
	if _, ok := cols[ProfileManual]; !ok {
		t.Fatal("postgres.columns must be in manual")
	}
}

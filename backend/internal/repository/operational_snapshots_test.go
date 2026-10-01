package repository

import "testing"

func TestCounterDeltasRespectStatsResetAndRegression(t *testing.T) {
	seq, idx, dml, reset := counterDeltas(12, 15, 20, 10, 11, 18, false)
	if reset || seq == nil || *seq != 2 || idx == nil || *idx != 4 || dml == nil || *dml != 2 {
		t.Fatalf("unexpected deltas: seq=%v idx=%v dml=%v reset=%v", seq, idx, dml, reset)
	}
	for _, tc := range []struct {
		seq, idx, dml int64
		resetChanged  bool
	}{
		{9, 15, 20, false}, {12, 10, 20, false}, {12, 15, 17, false}, {12, 15, 20, true},
	} {
		a, b, c, gotReset := counterDeltas(tc.seq, tc.idx, tc.dml, 10, 11, 18, tc.resetChanged)
		if !gotReset || a != nil || b != nil || c != nil {
			t.Fatalf("reset must omit deltas: %#v", tc)
		}
	}
}

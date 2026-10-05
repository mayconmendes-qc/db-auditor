package repository

import "testing"

func TestScopeScoreGolden(t *testing.T) {
	tests := []struct {
		name         string
		observations [][2]string
		score        int
		penalties    []int
	}{
		{"empty", nil, 100, nil},
		{"two categories", [][2]string{{"security", "critical"}, {"security", "high"}, {"storage", "medium"}}, 76, []int{40, 8}},
		{"bounded", [][2]string{{"security", "critical"}, {"security", "critical"}, {"security", "critical"}, {"security", "critical"}, {"security", "critical"}}, 0, []int{100}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			categories, score := calculateScopeCategories(tc.observations)
			if score != tc.score {
				t.Fatalf("score=%d want %d", score, tc.score)
			}
			for i, want := range tc.penalties {
				if categories[i].Penalty != want {
					t.Fatalf("category %d penalty=%d want %d", i, categories[i].Penalty, want)
				}
			}
		})
	}
}

func TestAggregateNullsScoreWhenIndexesMissing(t *testing.T) {
	got := AggregateScopeScores("db", "public", [][2]string{{"security", "low"}}, 3, true, false)
	if got.Score != nil || got.Status != "insufficient_coverage" {
		t.Fatalf("missing indexes must not look like 100: %+v", got)
	}
	ok := AggregateScopeScores("db", "public", nil, 2, false, true)
	if ok.Score == nil || *ok.Score != 100 || ok.Confidence != 0.75 {
		t.Fatalf("covered partial score: %+v", ok)
	}
	empty := AggregateScopeScores("db", "", nil, 0, false, false)
	if empty.Score != nil {
		t.Fatalf("no tables must not score 100")
	}
}

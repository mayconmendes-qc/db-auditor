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

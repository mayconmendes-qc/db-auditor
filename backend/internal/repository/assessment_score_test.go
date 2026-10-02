package repository

import "testing"

func TestCalculateStructuralScoreGolden(t *testing.T) {
	tests := []struct {
		name    string
		input   structuralScoreInput
		want    float64
		factors int
	}{
		{"healthy", structuralScoreInput{RelationKind: "r", HasPrimaryKey: true, LiveTuples: 1000}, 100, 0},
		{"structural risks", structuralScoreInput{RelationKind: "r", InvalidIndexes: 2, UnvalidatedConstraints: 1, LiveTuples: 600, DeadTuples: 400}, 30, 4},
		{"foreign table without key", structuralScoreInput{RelationKind: "f"}, 100, 0},
		{"child partition without key", structuralScoreInput{RelationKind: "r", IsPartition: true}, 100, 0},
		{"small table without key", structuralScoreInput{RelationKind: "r", LiveTuples: 10, DeadTuples: 9}, 80, 1},
		{"penalties are bounded", structuralScoreInput{RelationKind: "r", InvalidIndexes: 50, UnvalidatedConstraints: 50, LiveTuples: 100, DeadTuples: 900}, 10, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, factors := calculateStructuralScore(tt.input)
			if got != tt.want || len(factors) != tt.factors {
				t.Fatalf("score=%v factors=%v; want %v and %d factors", got, factors, tt.want, tt.factors)
			}
		})
	}
}

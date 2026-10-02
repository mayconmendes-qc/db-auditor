package repository

// The structural score deliberately uses only immutable snapshots from the
// requested run. Mutable, deduplicated findings are displayed alongside it,
// but cannot be an input until historical finding versions exist.
const structuralScoreVersion = "structural-v1"

var structuralScoreCollectors = []string{
	"postgres.tables",
	"postgres.columns",
	"postgres.constraints",
	"postgres.indexes",
}

type AssessmentScoreFactor struct {
	Code        string `json:"code"`
	Description string `json:"description"`
	Penalty     int    `json:"penalty"`
}

type structuralScoreInput struct {
	RelationKind           string
	IsPartition            bool
	HasPrimaryKey          bool
	LiveTuples             int64
	DeadTuples             int64
	InvalidIndexes         int
	UnvalidatedConstraints int
}

func calculateStructuralScore(input structuralScoreInput) (float64, []AssessmentScoreFactor) {
	score := 100
	factors := make([]AssessmentScoreFactor, 0, 4)
	add := func(code, description string, penalty int) {
		if penalty == 0 {
			return
		}
		score -= penalty
		factors = append(factors, AssessmentScoreFactor{Code: code, Description: description, Penalty: penalty})
	}
	// A child partition and a foreign table cannot always own a primary key.
	if !input.IsPartition && input.RelationKind != "f" && !input.HasPrimaryKey {
		add("missing_primary_key", "Tabela sem chave primária", 20)
	}
	if input.InvalidIndexes > 0 {
		add("invalid_indexes", "Índices inválidos no snapshot", min(input.InvalidIndexes, 2)*15)
	}
	if input.UnvalidatedConstraints > 0 {
		add("unvalidated_constraints", "Constraints não validadas no snapshot", min(input.UnvalidatedConstraints, 2)*10)
	}
	totalTuples := float64(input.LiveTuples) + float64(input.DeadTuples)
	if totalTuples >= 1000 {
		deadRatio := float64(input.DeadTuples) / totalTuples
		switch {
		case deadRatio > 0.5:
			add("dead_tuples", "Mais de 50% das tuplas estimadas estão mortas", 20)
		case deadRatio > 0.2:
			add("dead_tuples", "Mais de 20% das tuplas estimadas estão mortas", 10)
		}
	}
	if score < 0 {
		score = 0
	}
	return float64(score), factors
}

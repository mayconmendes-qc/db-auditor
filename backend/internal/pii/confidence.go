package pii

// ConfidenceFromSample implements spec 05 §3:
// high ≥90% valid and ≥50 non-null; medium 50–90% with ≥50 non-null; else low.
func ConfidenceFromSample(valid, nonNull, sampleSize int) string {
	if sampleSize <= 0 || nonNull < 50 {
		return ConfidenceBaixa
	}
	pct := float64(valid) / float64(nonNull) * 100
	if pct >= 90 {
		return ConfidenceAlta
	}
	if pct >= 50 {
		return ConfidenceMedia
	}
	return ConfidenceBaixa
}

// ConfidenceFromMetadata implements stage-1 confidence (spec 05 §3).
func ConfidenceFromMetadata(category, dataType string) string {
	if CompatibleType(category, dataType) {
		return ConfidenceMedia
	}
	return ConfidenceBaixa
}

// MergeConfidence picks the higher of two confidence labels.
func MergeConfidence(a, b string) string {
	rank := map[string]int{ConfidenceBaixa: 1, ConfidenceMedia: 2, ConfidenceAlta: 3}
	if rank[b] > rank[a] {
		return b
	}
	if a == "" {
		return b
	}
	return a
}

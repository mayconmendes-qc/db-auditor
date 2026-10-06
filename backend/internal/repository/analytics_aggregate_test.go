package repository

import "testing"

func TestTrendComparabilityRejectsChangedCollectionAndRules(t *testing.T) {
	points := []RunTrendPoint{
		{EnvironmentID: "one", Coverage: "complete", Profile: "full", CollectorVersion: "1", RuleVersion: "a"},
		{EnvironmentID: "one", Coverage: "complete", Profile: "full", CollectorVersion: "1", RuleVersion: "a"},
		{EnvironmentID: "one", Coverage: "partial", Profile: "full", CollectorVersion: "1", RuleVersion: "a"},
		{EnvironmentID: "one", Coverage: "complete", Profile: "full", CollectorVersion: "1", RuleVersion: "a"},
		{EnvironmentID: "one", Coverage: "complete", Profile: "full", CollectorVersion: "2", RuleVersion: "a"},
		{EnvironmentID: "one", Coverage: "complete", Profile: "full", CollectorVersion: "2", RuleVersion: "b"},
	}
	markTrendComparability(points)
	for i, expected := range []bool{true, true, false, false, false, false} {
		if points[i].Comparable != expected {
			t.Fatalf("point %d comparable=%t, expected %t", i, points[i].Comparable, expected)
		}
	}
	if points[3].ComparisonNote != "Coleta anterior parcial" || points[4].ComparisonNote != "Versão do coletor diferente" || points[5].ComparisonNote != "Versão das regras diferente" {
		t.Fatalf("unexpected comparison reasons: %+v", points)
	}
}

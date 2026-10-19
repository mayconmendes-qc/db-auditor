package dqcontract

import (
	"strings"
	"testing"
)

func TestGateExitHighUniqueViolation(t *testing.T) {
	results := []CheckResult{{
		CheckKey: "uniqueness:id", CheckType: CheckUniqueness, Result: ResultViolated, Severity: SeverityHigh,
	}}
	if GateExit(results, "high") != 1 {
		t.Fatal("expected exit 1")
	}
	if GateExit(results, "critical") != 0 {
		t.Fatal("critical gate should not block high")
	}
}

func TestGateExitTimeoutNotExecuted(t *testing.T) {
	results := []CheckResult{{
		CheckKey: "nulls:id", CheckType: CheckNulls, Result: ResultNotExecuted, Reason: "timeout", Severity: SeverityInfo,
	}}
	if GateExit(results, "high") != 3 {
		t.Fatal("expected exit 3")
	}
}

func TestRenderMarkdownListsViolation(t *testing.T) {
	md := RenderMarkdown(RunReport{
		EnvironmentID: "e", Database: "d", Schema: "s", Table: "t", SampleLimit: 10000,
		Mode: ModeCLI, Status: "success", Coverage: "complete",
		Results: []CheckResult{{
			CheckKey: "uniqueness:id", Result: ResultViolated, Severity: SeverityHigh, Reason: "duplicata na amostra",
		}},
	})
	if !strings.Contains(md, "uniqueness:id") || !strings.Contains(md, "violated") {
		t.Fatalf("markdown missing violation:\n%s", md)
	}
}

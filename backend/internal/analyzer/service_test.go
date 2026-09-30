package analyzer

import (
	"context"
	"errors"
	"testing"
)

type serviceStub struct {
	facts      SnapshotFacts
	saveErr    error
	started    bool
	finished   string
	finishErr  string
	savedCount int
}

func (s *serviceStub) LoadSnapshotFacts(context.Context, string, string) (SnapshotFacts, error) {
	return s.facts, nil
}
func (s *serviceStub) StartAnalysisRun(context.Context, string, string, string) error {
	s.started = true
	return nil
}
func (s *serviceStub) SaveAnalysisFindings(_ context.Context, findings []Finding) (int, error) {
	if s.saveErr != nil {
		return 0, s.saveErr
	}
	s.savedCount = len(findings)
	return len(findings), nil
}
func (s *serviceStub) FinishAnalysisRun(_ context.Context, _ string, status string, _, _ int, errMsg string) error {
	s.finished, s.finishErr = status, errMsg
	return nil
}

func TestServiceRecordsSuccessfulLifecycle(t *testing.T) {
	stub := &serviceStub{facts: SnapshotFacts{EnvironmentID: "env", AuditRunID: "run"}}
	produced, saved, err := NewService(stub, stub, "test").AnalyzeRun(context.Background(), "env", "run")
	if err != nil {
		t.Fatal(err)
	}
	if !stub.started || stub.finished != "success" {
		t.Fatalf("lifecycle = started:%v finished:%s", stub.started, stub.finished)
	}
	if produced != saved {
		t.Fatalf("produced=%d saved=%d", produced, saved)
	}
}

func TestServiceFailsWhenAtomicSaveFails(t *testing.T) {
	stub := &serviceStub{facts: SnapshotFacts{EnvironmentID: "env", AuditRunID: "run"}, saveErr: errors.New("write failed")}
	_, _, err := NewService(stub, stub, "test").AnalyzeRun(context.Background(), "env", "run")
	if err == nil || stub.finished != "failed" || stub.finishErr == "" {
		t.Fatalf("err=%v lifecycle=%s", err, stub.finished)
	}
}

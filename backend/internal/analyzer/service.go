package analyzer

import (
	"context"
	"fmt"
)

// SnapshotLoader loads facts from one immutable audit run.
type SnapshotLoader interface {
	LoadSnapshotFacts(ctx context.Context, environmentID, auditRunID string) (SnapshotFacts, error)
}

// FindingSink records analysis lifecycle and persists a complete result atomically.
type FindingSink interface {
	StartAnalysisRun(ctx context.Context, environmentID, auditRunID, version string) error
	SaveAnalysisFindings(ctx context.Context, findings []Finding) (int, error)
	FinishAnalysisRun(ctx context.Context, auditRunID, status string, produced, saved int, errMsg string) error
}

// Service executes analyzers only from server-side snapshots.
type Service struct {
	loader  SnapshotLoader
	sink    FindingSink
	runner  *Runner
	version string
}

func NewService(loader SnapshotLoader, sink FindingSink, version string) *Service {
	if version == "" {
		version = "1.0.0"
	}
	return &Service{loader: loader, sink: sink, runner: NewRunner(DefaultRegistry()), version: version}
}

func (s *Service) AnalyzeRun(ctx context.Context, environmentID, auditRunID string) (produced, saved int, retErr error) {
	if environmentID == "" || auditRunID == "" {
		return 0, 0, fmt.Errorf("environment id and audit run id are required")
	}
	if err := s.sink.StartAnalysisRun(ctx, environmentID, auditRunID, s.version); err != nil {
		return 0, 0, fmt.Errorf("start analysis: %w", err)
	}
	defer func() {
		finishCtx := context.WithoutCancel(ctx)
		status, errMsg := "success", ""
		if retErr != nil {
			status, errMsg = "failed", retErr.Error()
		}
		if err := s.sink.FinishAnalysisRun(finishCtx, auditRunID, status, produced, saved, errMsg); err != nil && retErr == nil {
			retErr = fmt.Errorf("finish analysis: %w", err)
		}
	}()

	facts, err := s.loader.LoadSnapshotFacts(ctx, environmentID, auditRunID)
	if err != nil {
		return 0, 0, fmt.Errorf("load snapshot facts: %w", err)
	}
	items, err := s.runner.Run(ctx, facts)
	if err != nil {
		return 0, 0, err
	}
	produced = len(items)
	saved, err = s.sink.SaveAnalysisFindings(ctx, items)
	if err != nil {
		return produced, 0, fmt.Errorf("save findings atomically: %w", err)
	}
	return produced, saved, nil
}

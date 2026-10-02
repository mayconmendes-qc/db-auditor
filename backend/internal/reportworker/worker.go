package reportworker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/mayconmendes-qc/db-auditor/internal/report"
	"github.com/mayconmendes-qc/db-auditor/internal/repository"
)

type Worker struct{ Store *repository.Store }

func (w Worker) Run(ctx context.Context) {
	if err := w.Store.RequeueInterruptedReports(ctx); err != nil {
		slog.Error("requeue interrupted reports", "error", err)
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	cleanup := time.NewTicker(time.Hour)
	defer cleanup.Stop()
	recovery := time.NewTicker(30 * time.Second)
	defer recovery.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.ProcessOne(ctx); err != nil {
				slog.Error("report worker", "error", err)
			}
		case <-cleanup.C:
			if _, err := w.Store.CleanupExpiredReports(ctx); err != nil {
				slog.Error("report cleanup", "error", err)
			}
		case <-recovery.C:
			if err := w.Store.RequeueInterruptedReports(ctx); err != nil {
				slog.Error("report recovery", "error", err)
			}
		}
	}
}

func (w Worker) ProcessOne(ctx context.Context) error {
	job, err := w.Store.ClaimReportJob(ctx)
	if err != nil || job == nil {
		return err
	}
	workCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	document, err := w.Store.LoadReportDocument(workCtx, *job)
	if err != nil {
		return w.fail(workCtx, job.ID, err)
	}
	document.RedactMetadata(os.Getenv("AUDITOR_REPORT_REDACT_METADATA"))
	pdf, err := report.RenderPDF(document)
	if err != nil {
		return w.fail(workCtx, job.ID, err)
	}
	if err = report.ValidatePDF(pdf); err != nil {
		return w.fail(workCtx, job.ID, err)
	}
	current, err := w.Store.GetReportJob(workCtx, job.EnvironmentID, job.ID)
	if err != nil {
		return err
	}
	if current == nil || current.Status == "cancelled" {
		return nil
	}
	digest := sha256.Sum256(pdf)
	artifact := repository.ReportArtifact{Content: pdf, SHA256: hex.EncodeToString(digest[:]), Filename: fmt.Sprintf("db-auditor-%s-%s.pdf", job.Type, job.ID), ContentType: "application/pdf"}
	err = w.Store.FinishReportJob(workCtx, job.ID, artifact)
	if errors.Is(err, repository.ErrReportConflict) {
		return nil
	}
	return err
}

func (w Worker) fail(ctx context.Context, id string, cause error) error {
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := w.Store.FailReportJob(finishCtx, id, cause); err != nil {
		return fmt.Errorf("report %s failed: %v; status update: %w", id, cause, err)
	}
	return fmt.Errorf("report %s failed: %w", id, cause)
}

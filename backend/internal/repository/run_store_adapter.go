package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/mayconmendes-qc/timescale-auditor/internal/database/sqlc"
)

// AuditRunStore adapts Store to audit.RunStore using sqlc queries.
type AuditRunStore struct {
	*Store
}

func parseUUID(id string) (pgtype.UUID, error) {
	var u pgtype.UUID
	if err := u.Scan(id); err != nil {
		return pgtype.UUID{}, fmt.Errorf("invalid uuid %q: %w", id, err)
	}
	return u, nil
}

func (a *AuditRunStore) StartAuditRun(ctx context.Context, environmentID, profile, serviceVersion, collectorVersion string) (string, error) {
	envID, err := parseUUID(environmentID)
	if err != nil {
		return "", err
	}
	run, err := a.q.CreateAuditRun(ctx, sqlc.CreateAuditRunParams{
		EnvironmentID:    envID,
		Profile:          profile,
		Status:           "running",
		ServiceVersion:   serviceVersion,
		CollectorVersion: collectorVersion,
		Warnings:         []byte("[]"),
		Errors:           []byte("[]"),
	})
	if err != nil {
		return "", err
	}
	return uuidString(run.ID), nil
}

func (a *AuditRunStore) FinishAuditRun(ctx context.Context, auditRunID, status string, warnings, errs []string) error {
	id, err := parseUUID(auditRunID)
	if err != nil {
		return err
	}
	w, err := json.Marshal(warnings)
	if err != nil {
		return err
	}
	e, err := json.Marshal(errs)
	if err != nil {
		return err
	}
	_, err = a.q.FinishAuditRun(ctx, sqlc.FinishAuditRunParams{
		ID: id, Status: status, Warnings: w, Errors: e,
	})
	return err
}

func (a *AuditRunStore) StartCollectorRun(ctx context.Context, auditRunID, name, version string) (string, error) {
	id, err := parseUUID(auditRunID)
	if err != nil {
		return "", err
	}
	run, err := a.q.CreateCollectorRun(ctx, sqlc.CreateCollectorRunParams{
		AuditRunID:       id,
		CollectorName:    name,
		CollectorVersion: version,
		Status:           "running",
		QueryName:        pgtype.Text{},
	})
	if err != nil {
		return "", err
	}
	return uuidString(run.ID), nil
}

func (a *AuditRunStore) FinishCollectorRun(ctx context.Context, collectorRunID, status string, rows int64, warning, errMsg string) error {
	id, err := parseUUID(collectorRunID)
	if err != nil {
		return err
	}
	_, err = a.q.FinishCollectorRun(ctx, sqlc.FinishCollectorRunParams{
		ID:            id,
		Status:        status,
		RowsCollected: rows,
		Warning:       textOrNull(warning),
		Error:         textOrNull(errMsg),
	})
	return err
}

func textOrNull(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

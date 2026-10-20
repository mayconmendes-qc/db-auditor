package main

import (
	"context"

	"github.com/osmendes/db-auditor/internal/repository"
	"github.com/osmendes/db-auditor/internal/scheduler"
)

// scheduleStoreAdapter bridges repository.ScheduleEntry and scheduler.Entry
// without an import cycle between those packages.
type scheduleStoreAdapter struct {
	store *repository.Store
}

func (a scheduleStoreAdapter) ListSchedules(ctx context.Context) ([]scheduler.Entry, error) {
	rows, err := a.store.ListSchedules(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]scheduler.Entry, len(rows))
	for i, row := range rows {
		out[i] = scheduler.Entry{
			EnvironmentID: row.EnvironmentID,
			Profile:       row.Profile,
			Enabled:       row.Enabled,
			LastRunAt:     row.LastRunAt,
			LastStatus:    row.LastStatus,
			NextRunAt:     row.NextRunAt,
		}
	}
	return out, nil
}

func (a scheduleStoreAdapter) SaveSchedule(ctx context.Context, entry scheduler.Entry) error {
	return a.store.SaveSchedule(ctx, repository.ScheduleEntry{
		EnvironmentID: entry.EnvironmentID,
		Profile:       entry.Profile,
		Enabled:       entry.Enabled,
		LastRunAt:     entry.LastRunAt,
		LastStatus:    entry.LastStatus,
		NextRunAt:     entry.NextRunAt,
	})
}

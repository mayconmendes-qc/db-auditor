package repository

import (
	"context"
	"time"
)

// SnapshotBackupAgeSeconds is how long since the last successful pg_dump of
// the snapshot store. With no successful backup the age is large and still
// grows, so a stalled job cannot look healthy.
func (s *Store) SnapshotBackupAgeSeconds(ctx context.Context, now time.Time) (float64, error) {
	var last *time.Time
	err := s.pool.QueryRow(ctx, `SELECT last_success_at FROM snapshot_backup WHERE id=1`).Scan(&last)
	if err != nil {
		// Missing table or row: treat as never backed up.
		return now.Sub(time.Unix(0, 0)).Seconds(), nil
	}
	if last == nil || last.IsZero() {
		return now.Sub(time.Unix(0, 0)).Seconds(), nil
	}
	age := now.Sub(last.UTC()).Seconds()
	if age < 0 {
		return 0, nil
	}
	return age, nil
}

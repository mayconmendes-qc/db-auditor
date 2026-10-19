package repository

import (
	"context"
	"log/slog"
	"strings"

	"github.com/osmendes/db-auditor/internal/analyzer"
	"github.com/osmendes/db-auditor/internal/config"
	"github.com/osmendes/db-auditor/internal/notify"
	"github.com/osmendes/db-auditor/internal/observability"
)

// NotifyDriftFindings delivers drift.* alerts to env destinations (fallback global).
// Dedup key = environment + object + change kind + run (ClaimAlert). Failures never fail the run.
func (s *Store) NotifyDriftFindings(ctx context.Context, findings []analyzer.Finding) {
	for _, f := range findings {
		if !strings.HasPrefix(f.FindingType, "drift.") {
			continue
		}
		fresh, err := s.ClaimAlert(ctx, f.EnvironmentID, f.DedupKey)
		if err != nil || !fresh {
			continue
		}
		changeKind, _ := f.Evidence["change_kind"].(string)
		event := notify.Event{
			Kind: f.FindingType, EnvironmentID: f.EnvironmentID, DedupKey: f.DedupKey,
			Title: f.Title, Severity: string(f.Severity),
		}
		drift := notify.DriftPayload{
			Event:      event,
			ObjectType: f.ObjectType,
			ObjectKey:  f.ObjectKey,
			ChangeKind: changeKind,
		}
		dests, err := s.ListActiveNotifyDestinations(ctx, f.EnvironmentID)
		if err != nil {
			slog.Warn("list drift destinations failed", "error", config.SanitizeError(err))
			dests = nil
		}
		if len(dests) == 0 {
			cfg := notify.FromEnv()
			if !cfg.Enabled() {
				continue
			}
			if err := cfg.Send(ctx, event); err != nil {
				observability.DefaultMetrics.IncAlertDeliveryFailed(f.FindingType)
				slog.Warn("drift alert global delivery failed", "error", config.SanitizeError(err))
			}
			continue
		}
		errs := notify.SendToDestinations(ctx, dests, event, drift)
		for _, e := range errs {
			if e != nil {
				observability.DefaultMetrics.IncAlertDeliveryFailed(f.FindingType)
				slog.Warn("drift alert delivery failed", "error", config.SanitizeError(e))
			}
		}
	}
}

// ListActiveNotifyDestinations returns decrypted destinations for delivery.
func (s *Store) ListActiveNotifyDestinations(ctx context.Context, env string) ([]notify.Destination, error) {
	rows, err := s.pool.Query(ctx, `
SELECT id::text, channel, target, format
FROM notification_destination
WHERE environment_id=$1::uuid AND active=true
ORDER BY created_at ASC, id ASC`, env)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []notify.Destination
	for rows.Next() {
		var d notify.Destination
		if err := rows.Scan(&d.ID, &d.Channel, &d.Target, &d.Format); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

package repository

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/osmendes/db-auditor/internal/config"
	"github.com/osmendes/db-auditor/internal/notify"
	"github.com/osmendes/db-auditor/internal/observability"
)

// DeliverQueryDigest sends the digest to env destinations or global fallback.
// Failures are recorded for retry; successful ClaimAlert prevents duplicates.
func (s *Store) DeliverQueryDigest(ctx context.Context, d *QueryDigest) error {
	if d == nil {
		return nil
	}
	already, err := s.DigestAlreadyDelivered(ctx, d.EnvironmentID, d.ID)
	if err != nil {
		return err
	}
	if already {
		return nil
	}

	secrets, err := s.listActiveDestinationSecrets(ctx, d.EnvironmentID)
	if err != nil {
		return err
	}
	event := notify.Event{
		Kind:          "query_digest",
		EnvironmentID: d.EnvironmentID,
		DedupKey:      d.ID,
		Title:         digestTitle(d),
		Severity:      "info",
	}
	link := digestLink(d.EnvironmentID, d.ID)
	payload := notify.DigestPayload{
		Event:            event,
		Link:             link,
		RegressionCount:  d.RegressionCount,
		NewCount:         d.NewCount,
		GoneCount:        d.GoneCount,
		ImprovementCount: d.ImprovementCount,
		State:            d.State,
	}

	allOK := true
	if len(secrets) == 0 {
		cfg := notify.FromEnv()
		if !cfg.Enabled() {
			observability.DefaultMetrics.IncQueryDigest(d.State)
			return nil
		}
		if err := cfg.Send(ctx, event); err != nil {
			allOK = false
			_ = s.RecordQueryDigestDelivery(ctx, d.ID, "", "global", "global", "failed", 1, config.SanitizeError(err))
			observability.DefaultMetrics.IncQueryDigestDeliveryFailed()
			slog.Warn("query digest global delivery failed", "environment_id", d.EnvironmentID, "error", config.SanitizeError(err))
		} else {
			_ = s.RecordQueryDigestDelivery(ctx, d.ID, "", "global", "global", "sent", 1, "")
		}
	} else {
		dests := make([]notify.Destination, 0, len(secrets))
		for _, sec := range secrets {
			dests = append(dests, notify.Destination{ID: sec.ID, Channel: sec.Channel, Target: sec.Target, Format: sec.Format})
		}
		errs := notify.SendToDestinations(ctx, dests, event, payload)
		for i, sec := range secrets {
			masked := MaskDestination(sec.Channel, sec.Target)
			if i < len(errs) && errs[i] != nil {
				allOK = false
				_ = s.RecordQueryDigestDelivery(ctx, d.ID, sec.ID, sec.Channel, masked, "failed", 1, config.SanitizeError(errs[i]))
				observability.DefaultMetrics.IncQueryDigestDeliveryFailed()
				slog.Warn("query digest delivery failed", "environment_id", d.EnvironmentID, "channel", sec.Channel, "error", config.SanitizeError(errs[i]))
			} else {
				_ = s.RecordQueryDigestDelivery(ctx, d.ID, sec.ID, sec.Channel, masked, "sent", 1, "")
			}
		}
	}

	if allOK {
		claimed, err := s.ClaimAlert(ctx, d.EnvironmentID, d.ID)
		if err != nil {
			return err
		}
		if !claimed {
			return nil
		}
	}
	observability.DefaultMetrics.IncQueryDigest(d.State)
	return nil
}

// RetryFailedQueryDigestDeliveries re-attempts failed deliveries for digests
// that were never ClaimAlert-ed (dedup_key = digest id).
func (s *Store) RetryFailedQueryDigestDeliveries(ctx context.Context, env string) error {
	rows, err := s.pool.Query(ctx, `SELECT DISTINCT d.id::text FROM query_digest d
JOIN query_digest_delivery del ON del.digest_id=d.id
WHERE d.environment_id=$1::uuid AND del.status='failed'
AND NOT EXISTS (SELECT 1 FROM alert_delivery a WHERE a.environment_id=d.environment_id AND a.dedup_key=d.id::text)
ORDER BY d.created_at DESC LIMIT 20`, env)
	if err != nil {
		return err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		d, err := s.GetQueryDigest(ctx, env, id)
		if err != nil || d == nil {
			continue
		}
		_ = s.DeliverQueryDigest(ctx, d)
	}
	return nil
}

func digestTitle(d *QueryDigest) string {
	return fmt.Sprintf("Digest de consultas (%s): %d regressões, %d novas, %d sumiram, %d melhoras",
		d.State, d.RegressionCount, d.NewCount, d.GoneCount, d.ImprovementCount)
}

func digestLink(env, id string) string {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("AUDITOR_PUBLIC_BASE_URL")), "/")
	if base == "" {
		return fmt.Sprintf("/digest?env=%s&id=%s", env, id)
	}
	return fmt.Sprintf("%s/#/digest?env=%s&id=%s", base, env, id)
}

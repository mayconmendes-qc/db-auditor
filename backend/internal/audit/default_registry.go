package audit

import "context"

// NewDefaultRegistry registers placeholder collectors representing the inventory set.
// Real connection-bound implementations are wired by the composition root when target DSNs exist.
func NewDefaultRegistry() *Registry {
	r := NewRegistry()
	placeholders := []string{
		"postgres.server",
		"postgres.databases",
		"postgres.schemas",
		"postgres.tables",
		"postgres.columns",
		"postgres.indexes",
		"postgres.constraints",
		"postgres.views",
		"postgres.functions",
		"postgres.extensions",
		"timescale.version",
		"timescale.hypertables",
		"timescale.dimensions",
		"timescale.chunks",
		"timescale.continuous_aggregates",
		"timescale.jobs",
		"timescale.policies",
	}
	for _, name := range placeholders {
		n := name
		_ = r.Register(CollectorSpec{
			Name:     n,
			Version:  "1.0.0",
			Profiles: DefaultProfiles(),
			Run: func(context.Context) (int64, error) {
				// Placeholder: no target connection yet; treat as skipped success with 0 rows.
				return 0, nil
			},
		})
	}
	return r
}

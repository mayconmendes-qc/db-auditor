package audit

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/mayconmendes-qc/db-auditor/internal/collectors/postgres"
	"github.com/mayconmendes-qc/db-auditor/internal/config"
)

// structuralPersister is implemented by repository.Store (SaveStructuralInventory).
type structuralPersister interface {
	SaveStructuralInventory(ctx context.Context, environmentID, auditRunID pgtype.UUID, sequences []postgres.SequenceFacts, triggers []postgres.TriggerFacts, policies []postgres.PolicyFacts) error
}

// AttachStructuralCollectors registers sequences/triggers/RLS policy collectors (Sprint 14).
func AttachStructuralCollectors(r *Registry, opts LiveRegistryOptions) {
	if r == nil {
		return
	}
	scope := opts.Scope
	targets := opts.Targets
	if targets == nil {
		targets = config.LoadTargetDSNs()
	}
	var writer structuralPersister
	if opts.Writer != nil {
		if w, ok := opts.Writer.(structuralPersister); ok {
			writer = w
		}
	}

	must := func(name string, fn CollectorFunc) {
		_ = r.Register(CollectorSpec{
			Name:     name,
			Version:  "1.0.0",
			Profiles: map[string]struct{}{ProfileManual: {}, ProfileMonthly: {}, ProfileWeekly: {}},
			Run:      fn,
		})
	}

	must("postgres.sequences", func(ctx context.Context) (int64, error) {
		dsn, err := dsnFromContext(ctx, targets)
		if err != nil {
			return 0, err
		}
		var all []postgres.SequenceFacts
		partial, hard := postgres.ForEachUserDatabase(ctx, dsn, scope, func(cctx context.Context, conn *pgx.Conn, _ string) error {
			items, err := postgres.CollectSequences(cctx, conn, scope)
			if err != nil {
				return err
			}
			all = append(all, items...)
			return nil
		})
		if hard != nil {
			return 0, hard
		}
		if writer != nil && len(all) > 0 {
			envID, runID, err := runIDs(ctx)
			if err != nil {
				return 0, err
			}
			if err := writer.SaveStructuralInventory(ctx, envID, runID, all, nil, nil); err != nil {
				return 0, fmt.Errorf("persistir sequences: %w", err)
			}
		}
		return finishMulti(int64(len(all)), partial, nil)
	})

	must("postgres.triggers", func(ctx context.Context) (int64, error) {
		dsn, err := dsnFromContext(ctx, targets)
		if err != nil {
			return 0, err
		}
		var all []postgres.TriggerFacts
		partial, hard := postgres.ForEachUserDatabase(ctx, dsn, scope, func(cctx context.Context, conn *pgx.Conn, _ string) error {
			items, err := postgres.CollectTriggers(cctx, conn, scope)
			if err != nil {
				return err
			}
			all = append(all, items...)
			return nil
		})
		if hard != nil {
			return 0, hard
		}
		if writer != nil && len(all) > 0 {
			envID, runID, err := runIDs(ctx)
			if err != nil {
				return 0, err
			}
			if err := writer.SaveStructuralInventory(ctx, envID, runID, nil, all, nil); err != nil {
				return 0, fmt.Errorf("persistir triggers: %w", err)
			}
		}
		return finishMulti(int64(len(all)), partial, nil)
	})

	must("postgres.policies", func(ctx context.Context) (int64, error) {
		dsn, err := dsnFromContext(ctx, targets)
		if err != nil {
			return 0, err
		}
		var all []postgres.PolicyFacts
		partial, hard := postgres.ForEachUserDatabase(ctx, dsn, scope, func(cctx context.Context, conn *pgx.Conn, _ string) error {
			items, err := postgres.CollectPolicies(cctx, conn, scope)
			if err != nil {
				return err
			}
			all = append(all, items...)
			return nil
		})
		if hard != nil {
			return 0, hard
		}
		if writer != nil && len(all) > 0 {
			envID, runID, err := runIDs(ctx)
			if err != nil {
				return 0, err
			}
			if err := writer.SaveStructuralInventory(ctx, envID, runID, nil, nil, all); err != nil {
				return 0, fmt.Errorf("persistir rls policies: %w", err)
			}
		}
		return finishMulti(int64(len(all)), partial, nil)
	})
}

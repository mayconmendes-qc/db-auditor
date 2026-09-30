package audit

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/mayconmendes-qc/db-auditor/internal/collectors/postgres"
	"github.com/mayconmendes-qc/db-auditor/internal/config"
)

func registerStructuralCollectors(
	_ *Registry,
	must func(name string, fn CollectorFunc),
	scope config.Scope,
	targets map[string]string,
	writer InventoryWriter,
) {
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

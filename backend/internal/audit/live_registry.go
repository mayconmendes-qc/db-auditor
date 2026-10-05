package audit

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/mayconmendes-qc/db-auditor/internal/collectors/postgres"
	"github.com/mayconmendes-qc/db-auditor/internal/collectors/timescale"
	"github.com/mayconmendes-qc/db-auditor/internal/config"
)

// InventoryWriter persists collector facts into snapshot tables.
// Implemented by repository.Store.
type InventoryWriter interface {
	SaveDiscovery(ctx context.Context, environmentID, auditRunID pgtype.UUID, databases []postgres.DatabaseFacts, schemas []postgres.SchemaFacts) error
	SaveObjectInventory(ctx context.Context, environmentID, auditRunID pgtype.UUID, tables []postgres.TableFacts, columns []postgres.ColumnFacts, indexes []postgres.IndexFacts) error
	SaveExtendedObjectInventory(ctx context.Context, environmentID, auditRunID pgtype.UUID, constraints []postgres.ConstraintFacts, views []postgres.ViewFacts, functions []postgres.FunctionFacts, extensions []postgres.ExtensionFacts) error
	SaveTimescaleCoreInventory(ctx context.Context, environmentID, auditRunID pgtype.UUID, result timescale.InventoryResult) error
	SaveTimescalePolicyInventory(ctx context.Context, environmentID, auditRunID pgtype.UUID, result timescale.PolicyInventoryResult) error
	SaveOperationalInventory(ctx context.Context, environmentID, auditRunID pgtype.UUID, columnStats []postgres.ColumnStatFacts, workload []postgres.WorkloadFacts) error
}

// LiveRegistryOptions configures collectors that hit audited databases.
type LiveRegistryOptions struct {
	Targets map[string]string
	Scope   config.Scope
	Policy  config.CollectionPolicy
	Writer  InventoryWriter
}

func parseRunUUID(s string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(s); err != nil {
		return id, fmt.Errorf("uuid inválido %q: %w", s, err)
	}
	return id, nil
}

func runIDs(ctx context.Context) (envID, runID pgtype.UUID, err error) {
	meta, ok := RunMetaFromContext(ctx)
	if !ok || meta.EnvironmentID == "" || meta.AuditRunID == "" {
		return envID, runID, fmt.Errorf("contexto da execução sem environment_id/audit_run_id")
	}
	envID, err = parseRunUUID(meta.EnvironmentID)
	if err != nil {
		return envID, runID, err
	}
	runID, err = parseRunUUID(meta.AuditRunID)
	if err != nil {
		return envID, runID, err
	}
	return envID, runID, nil
}

func dsnFromContext(ctx context.Context, targets map[string]string) (string, error) {
	meta, ok := RunMetaFromContext(ctx)
	if !ok || meta.EnvironmentID == "" {
		return "", fmt.Errorf("contexto da execução sem environment_id")
	}
	dsn := config.DSNForEnvironment(targets, meta.EnvironmentID)
	if dsn == "" {
		return "", fmt.Errorf(
			"credenciais do ambiente não configuradas. Defina %s no .env (somente leitura) e reinicie a API",
			config.TargetSlotHint(meta.EnvironmentID),
		)
	}
	return dsn, nil
}

// finishMulti returns rows and either nil, *PartialWarning, or a hard error from listing DBs.
// Per-database failures never abort the audit run: they become warnings on the collector.
func finishMulti(rows int64, partial []postgres.PartialError, hard error) (int64, error) {
	if hard != nil {
		return 0, hard
	}
	if len(partial) == 0 {
		return rows, nil
	}
	msg := postgres.FormatPartialErrors(partial, 5)
	failures := make([]CoverageFailure, 0, len(partial))
	for _, item := range partial {
		failures = append(failures, CoverageFailure{Database: item.Database, Error: item.Message})
	}
	return rows, &PartialWarning{
		Rows:     rows,
		Warning:  fmt.Sprintf("%d database(s) com falha parcial: %s", len(partial), msg),
		Failures: failures,
	}
}

// NewLiveRegistry registers collectors that connect using AUDITOR_TARGET_N_* credentials
// and optionally persist inventory snapshots when Writer is set.
// Object collectors iterate every connectable database, continuing on per-DB errors.
func NewLiveRegistry(opts LiveRegistryOptions) *Registry {
	r := NewRegistry()
	scope := opts.Scope
	targets := opts.Targets
	if targets == nil {
		targets = config.LoadTargetDSNs()
	}
	writer := opts.Writer

	connectRoot := func(ctx context.Context) (*pgx.Conn, error) {
		dsn, err := dsnFromContext(ctx, targets)
		if err != nil {
			return nil, err
		}
		conn, err := pgx.Connect(ctx, dsn)
		if err != nil {
			return nil, fmt.Errorf("conectar ao ambiente auditado: %w", err)
		}
		return conn, nil
	}

	must := func(name string, fn CollectorFunc) {
		_ = r.Register(CollectorSpec{
			Name:     name,
			Version:  "1.0.0",
			Profiles: liveCollectorProfiles(name),
			Run:      fn,
		})
	}

	must("postgres.server", func(ctx context.Context) (int64, error) {
		conn, err := connectRoot(ctx)
		if err != nil {
			return 0, err
		}
		defer func() { _ = conn.Close(ctx) }()
		if _, err := postgres.CollectServer(ctx, conn); err != nil {
			return 0, err
		}
		role, err := postgres.CollectAuditorRole(ctx, conn)
		if err != nil {
			return 0, err
		}
		if saver, ok := writer.(interface {
			SaveAuditorPrivilege(context.Context, pgtype.UUID, pgtype.UUID, postgres.AuditorRole) error
		}); ok {
			envID, runID, err := runIDs(ctx)
			if err != nil {
				return 0, err
			}
			if err = saver.SaveAuditorPrivilege(ctx, envID, runID, role); err != nil {
				return 0, err
			}
		}
		if settings, err := postgres.CollectClosedSettings(ctx, conn); err == nil && len(settings) > 0 {
			if saver, ok := writer.(interface {
				SaveServerSettings(context.Context, pgtype.UUID, pgtype.UUID, string, map[string]string) error
			}); ok {
				envID, runID, err := runIDs(ctx)
				if err != nil {
					return 0, err
				}
				if err = saver.SaveServerSettings(ctx, envID, runID, role.Database, settings); err != nil {
					return 0, err
				}
			}
		}
		return 1, nil
	})

	must("postgres.databases", func(ctx context.Context) (int64, error) {
		conn, err := connectRoot(ctx)
		if err != nil {
			return 0, err
		}
		defer func() { _ = conn.Close(ctx) }()
		dbs, err := postgres.CollectDatabases(ctx, conn, scope)
		if err != nil {
			return 0, err
		}
		if writer != nil && len(dbs) > 0 {
			envID, runID, err := runIDs(ctx)
			if err != nil {
				return 0, err
			}
			if err := writer.SaveDiscovery(ctx, envID, runID, dbs, nil); err != nil {
				return 0, fmt.Errorf("persistir databases: %w", err)
			}
		}
		return int64(len(dbs)), nil
	})

	must("postgres.schemas", func(ctx context.Context) (int64, error) {
		dsn, err := dsnFromContext(ctx, targets)
		if err != nil {
			return 0, err
		}
		var all []postgres.SchemaFacts
		partial, hard := postgres.ForEachUserDatabase(ctx, dsn, scope, func(cctx context.Context, conn *pgx.Conn, _ string) error {
			items, err := postgres.CollectSchemas(cctx, conn, scope)
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
			if err := writer.SaveDiscovery(ctx, envID, runID, nil, all); err != nil {
				return 0, fmt.Errorf("persistir schemas: %w", err)
			}
		}
		return finishMulti(int64(len(all)), partial, nil)
	})

	must("postgres.tables", func(ctx context.Context) (int64, error) {
		dsn, err := dsnFromContext(ctx, targets)
		if err != nil {
			return 0, err
		}
		var all []postgres.TableFacts
		partial, hard := postgres.ForEachUserDatabase(ctx, dsn, scope, func(cctx context.Context, conn *pgx.Conn, _ string) error {
			items, err := postgres.CollectTables(cctx, conn, scope)
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
			if err := writer.SaveObjectInventory(ctx, envID, runID, all, nil, nil); err != nil {
				return 0, fmt.Errorf("persistir tables: %w", err)
			}
		}
		return finishMulti(int64(len(all)), partial, nil)
	})

	must("postgres.columns", func(ctx context.Context) (int64, error) {
		dsn, err := dsnFromContext(ctx, targets)
		if err != nil {
			return 0, err
		}
		var all []postgres.ColumnFacts
		partial, hard := postgres.ForEachUserDatabase(ctx, dsn, scope, func(cctx context.Context, conn *pgx.Conn, _ string) error {
			items, err := postgres.CollectColumns(cctx, conn, scope)
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
			if err := writer.SaveObjectInventory(ctx, envID, runID, nil, all, nil); err != nil {
				return 0, fmt.Errorf("persistir columns: %w", err)
			}
		}
		return finishMulti(int64(len(all)), partial, nil)
	})

	must("postgres.indexes", func(ctx context.Context) (int64, error) {
		dsn, err := dsnFromContext(ctx, targets)
		if err != nil {
			return 0, err
		}
		var all []postgres.IndexFacts
		partial, hard := postgres.ForEachUserDatabase(ctx, dsn, scope, func(cctx context.Context, conn *pgx.Conn, _ string) error {
			items, err := postgres.CollectIndexes(cctx, conn, scope)
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
			if err := writer.SaveObjectInventory(ctx, envID, runID, nil, nil, all); err != nil {
				return 0, fmt.Errorf("persistir indexes: %w", err)
			}
		}
		return finishMulti(int64(len(all)), partial, nil)
	})

	must("postgres.constraints", func(ctx context.Context) (int64, error) {
		dsn, err := dsnFromContext(ctx, targets)
		if err != nil {
			return 0, err
		}
		var all []postgres.ConstraintFacts
		partial, hard := postgres.ForEachUserDatabase(ctx, dsn, scope, func(cctx context.Context, conn *pgx.Conn, _ string) error {
			items, err := postgres.CollectConstraints(cctx, conn, scope)
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
			if err := writer.SaveExtendedObjectInventory(ctx, envID, runID, all, nil, nil, nil); err != nil {
				return 0, fmt.Errorf("persistir constraints: %w", err)
			}
		}
		return finishMulti(int64(len(all)), partial, nil)
	})

	must("postgres.views", func(ctx context.Context) (int64, error) {
		dsn, err := dsnFromContext(ctx, targets)
		if err != nil {
			return 0, err
		}
		var all []postgres.ViewFacts
		partial, hard := postgres.ForEachUserDatabase(ctx, dsn, scope, func(cctx context.Context, conn *pgx.Conn, _ string) error {
			items, err := postgres.CollectViews(cctx, conn, scope)
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
			if err := writer.SaveExtendedObjectInventory(ctx, envID, runID, nil, all, nil, nil); err != nil {
				return 0, fmt.Errorf("persistir views: %w", err)
			}
		}
		return finishMulti(int64(len(all)), partial, nil)
	})

	must("postgres.functions", func(ctx context.Context) (int64, error) {
		dsn, err := dsnFromContext(ctx, targets)
		if err != nil {
			return 0, err
		}
		var all []postgres.FunctionFacts
		partial, hard := postgres.ForEachUserDatabase(ctx, dsn, scope, func(cctx context.Context, conn *pgx.Conn, _ string) error {
			items, err := postgres.CollectFunctions(cctx, conn, scope)
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
			if err := writer.SaveExtendedObjectInventory(ctx, envID, runID, nil, nil, all, nil); err != nil {
				return 0, fmt.Errorf("persistir functions: %w", err)
			}
		}
		return finishMulti(int64(len(all)), partial, nil)
	})

	must("postgres.extensions", func(ctx context.Context) (int64, error) {
		dsn, err := dsnFromContext(ctx, targets)
		if err != nil {
			return 0, err
		}
		var all []postgres.ExtensionFacts
		partial, hard := postgres.ForEachUserDatabase(ctx, dsn, scope, func(cctx context.Context, conn *pgx.Conn, _ string) error {
			items, err := postgres.CollectExtensions(cctx, conn)
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
			if err := writer.SaveExtendedObjectInventory(ctx, envID, runID, nil, nil, nil, all); err != nil {
				return 0, fmt.Errorf("persistir extensions: %w", err)
			}
		}
		return finishMulti(int64(len(all)), partial, nil)
	})

	must("timescale.version", func(ctx context.Context) (int64, error) {
		dsn, err := dsnFromContext(ctx, targets)
		if err != nil {
			return 0, err
		}
		var n int64
		partial, hard := postgres.ForEachUserDatabase(ctx, dsn, scope, func(cctx context.Context, conn *pgx.Conn, _ string) error {
			v, err := timescale.CollectVersion(cctx, conn)
			if err != nil {
				return err
			}
			if v == nil {
				return nil
			}
			n++
			if writer != nil {
				envID, runID, err := runIDs(ctx)
				if err != nil {
					return err
				}
				status := timescale.StatusOK
				if !v.Compatible {
					status = timescale.StatusSkippedUnsupported
				}
				if err := writer.SaveTimescaleCoreInventory(ctx, envID, runID, timescale.InventoryResult{
					Status:  status,
					Version: v,
				}); err != nil {
					return fmt.Errorf("persistir timescale version: %w", err)
				}
			}
			return nil
		})
		if hard != nil {
			return 0, hard
		}
		return finishMulti(n, partial, nil)
	})

	must("timescale.hypertables", func(ctx context.Context) (int64, error) {
		dsn, err := dsnFromContext(ctx, targets)
		if err != nil {
			return 0, err
		}
		var all []timescale.HypertableFacts
		partial, hard := postgres.ForEachUserDatabase(ctx, dsn, scope, func(cctx context.Context, conn *pgx.Conn, _ string) error {
			items, err := timescale.CollectHypertables(cctx, conn, scope)
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
			if err := writer.SaveTimescaleCoreInventory(ctx, envID, runID, timescale.InventoryResult{
				Status:      timescale.StatusOK,
				Hypertables: all,
			}); err != nil {
				return 0, fmt.Errorf("persistir hypertables: %w", err)
			}
		}
		return finishMulti(int64(len(all)), partial, nil)
	})

	must("timescale.dimensions", func(ctx context.Context) (int64, error) {
		dsn, err := dsnFromContext(ctx, targets)
		if err != nil {
			return 0, err
		}
		var all []timescale.DimensionFacts
		partial, hard := postgres.ForEachUserDatabase(ctx, dsn, scope, func(cctx context.Context, conn *pgx.Conn, _ string) error {
			items, err := timescale.CollectDimensions(cctx, conn, scope)
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
			if err := writer.SaveTimescaleCoreInventory(ctx, envID, runID, timescale.InventoryResult{
				Status:     timescale.StatusOK,
				Dimensions: all,
			}); err != nil {
				return 0, fmt.Errorf("persistir dimensions: %w", err)
			}
		}
		return finishMulti(int64(len(all)), partial, nil)
	})

	must("timescale.chunks", func(ctx context.Context) (int64, error) {
		dsn, err := dsnFromContext(ctx, targets)
		if err != nil {
			return 0, err
		}
		var all []timescale.ChunkFacts
		var truncated []postgres.PartialError
		partial, hard := postgres.ForEachUserDatabase(ctx, dsn, scope, func(cctx context.Context, conn *pgx.Conn, _ string) error {
			items, err := timescale.CollectChunks(cctx, conn, scope)
			if err != nil && !strings.Contains(err.Error(), "chunk inventory truncated") {
				return err
			}
			if err != nil {
				truncated = append(truncated, postgres.PartialError{Database: "chunks", Op: "chunks", Message: "timescale.chunks: truncated: chunk inventory over the 2000 row cap; coverage is incomplete"})
			}
			all = append(all, items...)
			return nil
		})
		partial = append(partial, truncated...)
		if hard != nil {
			return 0, hard
		}
		if writer != nil && len(all) > 0 {
			envID, runID, err := runIDs(ctx)
			if err != nil {
				return 0, err
			}
			if err := writer.SaveTimescaleCoreInventory(ctx, envID, runID, timescale.InventoryResult{
				Status: timescale.StatusOK,
				Chunks: all,
			}); err != nil {
				return 0, fmt.Errorf("persistir chunks: %w", err)
			}
		}
		return finishMulti(int64(len(all)), partial, nil)
	})

	must("timescale.continuous_aggregates", func(ctx context.Context) (int64, error) {
		dsn, err := dsnFromContext(ctx, targets)
		if err != nil {
			return 0, err
		}
		var all []timescale.ContinuousAggregateFacts
		partial, hard := postgres.ForEachUserDatabase(ctx, dsn, scope, func(cctx context.Context, conn *pgx.Conn, _ string) error {
			items, err := timescale.CollectContinuousAggregates(cctx, conn, scope)
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
			if err := writer.SaveTimescalePolicyInventory(ctx, envID, runID, timescale.PolicyInventoryResult{
				Status:               timescale.StatusOK,
				ContinuousAggregates: all,
			}); err != nil {
				return 0, fmt.Errorf("persistir continuous aggregates: %w", err)
			}
		}
		return finishMulti(int64(len(all)), partial, nil)
	})

	must("timescale.jobs", func(ctx context.Context) (int64, error) {
		dsn, err := dsnFromContext(ctx, targets)
		if err != nil {
			return 0, err
		}
		var all []timescale.JobFacts
		partial, hard := postgres.ForEachUserDatabase(ctx, dsn, scope, func(cctx context.Context, conn *pgx.Conn, _ string) error {
			items, err := timescale.CollectJobs(cctx, conn, scope)
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
			if err := writer.SaveTimescalePolicyInventory(ctx, envID, runID, timescale.PolicyInventoryResult{
				Status: timescale.StatusOK,
				Jobs:   all,
			}); err != nil {
				return 0, fmt.Errorf("persistir jobs: %w", err)
			}
		}
		return finishMulti(int64(len(all)), partial, nil)
	})

	must("timescale.policies", func(ctx context.Context) (int64, error) {
		dsn, err := dsnFromContext(ctx, targets)
		if err != nil {
			return 0, err
		}
		var all []timescale.PolicyFacts
		partial, hard := postgres.ForEachUserDatabase(ctx, dsn, scope, func(cctx context.Context, conn *pgx.Conn, _ string) error {
			items, err := timescale.CollectPolicies(cctx, conn, scope)
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
			if err := writer.SaveTimescalePolicyInventory(ctx, envID, runID, timescale.PolicyInventoryResult{
				Status:   timescale.StatusOK,
				Policies: all,
			}); err != nil {
				return 0, fmt.Errorf("persistir policies: %w", err)
			}
		}
		return finishMulti(int64(len(all)), partial, nil)
	})

	if opts.Policy.ColumnStatsEnabled {
		must("postgres.column_stats", func(ctx context.Context) (int64, error) {
			dsn, err := dsnFromContext(ctx, targets)
			if err != nil {
				return 0, err
			}
			var all []postgres.ColumnStatFacts
			partial, hard := postgres.ForEachUserDatabase(ctx, dsn, scope, func(cctx context.Context, conn *pgx.Conn, _ string) error {
				items, err := postgres.CollectColumnStatsWithBudget(cctx, conn, scope, opts.Policy.ColumnStatsSchemas, opts.Policy.ColumnStatsLimit, opts.Policy.OptionalMaxPlanCost)
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
				if err := writer.SaveOperationalInventory(ctx, envID, runID, all, nil); err != nil {
					return 0, fmt.Errorf("persistir estatísticas de colunas: %w", err)
				}
			}
			return finishMulti(int64(len(all)), partial, nil)
		})
	}

	if opts.Policy.WorkloadEnabled {
		must("postgres.workload", func(ctx context.Context) (int64, error) {
			dsn, err := dsnFromContext(ctx, targets)
			if err != nil {
				return 0, err
			}
			var all []postgres.WorkloadFacts
			unavailable := 0
			partial, hard := postgres.ForEachUserDatabase(ctx, dsn, scope, func(cctx context.Context, conn *pgx.Conn, _ string) error {
				items, available, err := postgres.CollectWorkloadWithBudget(cctx, conn, opts.Policy.WorkloadLimit, opts.Policy.OptionalMaxPlanCost)
				if err != nil {
					return err
				}
				if !available {
					unavailable++
					return nil
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
				if err := writer.SaveOperationalInventory(ctx, envID, runID, nil, all); err != nil {
					return 0, fmt.Errorf("persistir workload: %w", err)
				}
			}
			rows, err := finishMulti(int64(len(all)), partial, nil)
			if unavailable > 0 {
				note := fmt.Sprintf("pg_stat_statements indisponível em %d database(s)", unavailable)
				if pw, ok := err.(*PartialWarning); ok {
					pw.Warning += "; " + note
					return rows, pw
				}
				return rows, &PartialWarning{Rows: rows, Warning: note}
			}
			return rows, err
		})
	}

	return r
}

func liveCollectorProfiles(name string) map[string]struct{} {
	profiles := map[string]struct{}{ProfileManual: {}, ProfileMonthly: {}}
	weekly := map[string]struct{}{
		"postgres.server": {}, "postgres.databases": {}, "postgres.schemas": {},
		"postgres.tables": {}, "postgres.columns": {}, "postgres.indexes": {},
		"postgres.constraints": {}, "postgres.views": {}, "postgres.functions": {},
		"postgres.extensions": {}, "timescale.version": {}, "timescale.hypertables": {},
		"timescale.dimensions": {}, "timescale.chunks": {}, "timescale.continuous_aggregates": {},
		"timescale.jobs": {}, "timescale.policies": {},
		"postgres.column_stats": {}, "postgres.workload": {},
	}
	daily := map[string]struct{}{
		"postgres.server": {}, "postgres.databases": {}, "postgres.schemas": {},
		"postgres.tables": {}, "postgres.indexes": {}, "timescale.version": {},
		"timescale.hypertables": {}, "timescale.jobs": {}, "timescale.policies": {},
		"postgres.workload": {},
	}
	if _, ok := weekly[name]; ok {
		profiles[ProfileWeekly] = struct{}{}
	}
	if _, ok := daily[name]; ok {
		profiles[ProfileDaily] = struct{}{}
	}
	if name == "postgres.server" || name == "postgres.databases" {
		profiles[ProfileFast] = struct{}{}
	}
	return profiles
}

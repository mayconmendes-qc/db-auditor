package audit

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/mayconmendes-qc/timescale-auditor/internal/collectors/postgres"
	"github.com/mayconmendes-qc/timescale-auditor/internal/collectors/timescale"
	"github.com/mayconmendes-qc/timescale-auditor/internal/config"
)

// InventoryWriter persists collector facts into snapshot tables.
// Implemented by repository.Store.
type InventoryWriter interface {
	SaveDiscovery(ctx context.Context, environmentID, auditRunID pgtype.UUID, databases []postgres.DatabaseFacts, schemas []postgres.SchemaFacts) error
	SaveObjectInventory(ctx context.Context, environmentID, auditRunID pgtype.UUID, tables []postgres.TableFacts, columns []postgres.ColumnFacts, indexes []postgres.IndexFacts) error
	SaveExtendedObjectInventory(ctx context.Context, environmentID, auditRunID pgtype.UUID, constraints []postgres.ConstraintFacts, views []postgres.ViewFacts, functions []postgres.FunctionFacts, extensions []postgres.ExtensionFacts) error
	SaveTimescaleCoreInventory(ctx context.Context, environmentID, auditRunID pgtype.UUID, result timescale.InventoryResult) error
	SaveTimescalePolicyInventory(ctx context.Context, environmentID, auditRunID pgtype.UUID, result timescale.PolicyInventoryResult) error
}

// LiveRegistryOptions configures collectors that hit audited databases.
type LiveRegistryOptions struct {
	Targets map[string]string
	Scope   config.Scope
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

// NewLiveRegistry registers collectors that connect using AUDITOR_TARGET_N_* credentials
// and optionally persist inventory snapshots when Writer is set.
func NewLiveRegistry(opts LiveRegistryOptions) *Registry {
	r := NewRegistry()
	scope := opts.Scope
	targets := opts.Targets
	if targets == nil {
		targets = config.LoadTargetDSNs()
	}
	writer := opts.Writer

	connect := func(ctx context.Context) (*pgx.Conn, error) {
		meta, ok := RunMetaFromContext(ctx)
		if !ok || meta.EnvironmentID == "" {
			return nil, fmt.Errorf("contexto da execução sem environment_id")
		}
		dsn := config.DSNForEnvironment(targets, meta.EnvironmentID)
		if dsn == "" {
			return nil, fmt.Errorf(
				"credenciais do ambiente não configuradas. Defina %s no .env (somente leitura) e reinicie a API",
				config.TargetSlotHint(meta.EnvironmentID),
			)
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
			Profiles: DefaultProfiles(),
			Run:      fn,
		})
	}

	must("postgres.server", func(ctx context.Context) (int64, error) {
		conn, err := connect(ctx)
		if err != nil {
			return 0, err
		}
		defer func() { _ = conn.Close(ctx) }()
		if _, err := postgres.CollectServer(ctx, conn); err != nil {
			return 0, err
		}
		return 1, nil
	})

	must("postgres.databases", func(ctx context.Context) (int64, error) {
		conn, err := connect(ctx)
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
		conn, err := connect(ctx)
		if err != nil {
			return 0, err
		}
		defer func() { _ = conn.Close(ctx) }()
		schemas, err := postgres.CollectSchemas(ctx, conn, scope)
		if err != nil {
			return 0, err
		}
		if writer != nil && len(schemas) > 0 {
			envID, runID, err := runIDs(ctx)
			if err != nil {
				return 0, err
			}
			if err := writer.SaveDiscovery(ctx, envID, runID, nil, schemas); err != nil {
				return 0, fmt.Errorf("persistir schemas: %w", err)
			}
		}
		return int64(len(schemas)), nil
	})

	must("postgres.tables", func(ctx context.Context) (int64, error) {
		conn, err := connect(ctx)
		if err != nil {
			return 0, err
		}
		defer func() { _ = conn.Close(ctx) }()
		tables, err := postgres.CollectTables(ctx, conn, scope)
		if err != nil {
			return 0, err
		}
		if writer != nil && len(tables) > 0 {
			envID, runID, err := runIDs(ctx)
			if err != nil {
				return 0, err
			}
			if err := writer.SaveObjectInventory(ctx, envID, runID, tables, nil, nil); err != nil {
				return 0, fmt.Errorf("persistir tables: %w", err)
			}
		}
		return int64(len(tables)), nil
	})

	must("postgres.columns", func(ctx context.Context) (int64, error) {
		conn, err := connect(ctx)
		if err != nil {
			return 0, err
		}
		defer func() { _ = conn.Close(ctx) }()
		cols, err := postgres.CollectColumns(ctx, conn, scope)
		if err != nil {
			return 0, err
		}
		if writer != nil && len(cols) > 0 {
			envID, runID, err := runIDs(ctx)
			if err != nil {
				return 0, err
			}
			if err := writer.SaveObjectInventory(ctx, envID, runID, nil, cols, nil); err != nil {
				return 0, fmt.Errorf("persistir columns: %w", err)
			}
		}
		return int64(len(cols)), nil
	})

	must("postgres.indexes", func(ctx context.Context) (int64, error) {
		conn, err := connect(ctx)
		if err != nil {
			return 0, err
		}
		defer func() { _ = conn.Close(ctx) }()
		idxs, err := postgres.CollectIndexes(ctx, conn, scope)
		if err != nil {
			return 0, err
		}
		if writer != nil && len(idxs) > 0 {
			envID, runID, err := runIDs(ctx)
			if err != nil {
				return 0, err
			}
			if err := writer.SaveObjectInventory(ctx, envID, runID, nil, nil, idxs); err != nil {
				return 0, fmt.Errorf("persistir indexes: %w", err)
			}
		}
		return int64(len(idxs)), nil
	})

	must("postgres.constraints", func(ctx context.Context) (int64, error) {
		conn, err := connect(ctx)
		if err != nil {
			return 0, err
		}
		defer func() { _ = conn.Close(ctx) }()
		items, err := postgres.CollectConstraints(ctx, conn, scope)
		if err != nil {
			return 0, err
		}
		if writer != nil && len(items) > 0 {
			envID, runID, err := runIDs(ctx)
			if err != nil {
				return 0, err
			}
			if err := writer.SaveExtendedObjectInventory(ctx, envID, runID, items, nil, nil, nil); err != nil {
				return 0, fmt.Errorf("persistir constraints: %w", err)
			}
		}
		return int64(len(items)), nil
	})

	must("postgres.views", func(ctx context.Context) (int64, error) {
		conn, err := connect(ctx)
		if err != nil {
			return 0, err
		}
		defer func() { _ = conn.Close(ctx) }()
		items, err := postgres.CollectViews(ctx, conn, scope)
		if err != nil {
			return 0, err
		}
		if writer != nil && len(items) > 0 {
			envID, runID, err := runIDs(ctx)
			if err != nil {
				return 0, err
			}
			if err := writer.SaveExtendedObjectInventory(ctx, envID, runID, nil, items, nil, nil); err != nil {
				return 0, fmt.Errorf("persistir views: %w", err)
			}
		}
		return int64(len(items)), nil
	})

	must("postgres.functions", func(ctx context.Context) (int64, error) {
		conn, err := connect(ctx)
		if err != nil {
			return 0, err
		}
		defer func() { _ = conn.Close(ctx) }()
		items, err := postgres.CollectFunctions(ctx, conn, scope)
		if err != nil {
			return 0, err
		}
		if writer != nil && len(items) > 0 {
			envID, runID, err := runIDs(ctx)
			if err != nil {
				return 0, err
			}
			if err := writer.SaveExtendedObjectInventory(ctx, envID, runID, nil, nil, items, nil); err != nil {
				return 0, fmt.Errorf("persistir functions: %w", err)
			}
		}
		return int64(len(items)), nil
	})

	must("postgres.extensions", func(ctx context.Context) (int64, error) {
		conn, err := connect(ctx)
		if err != nil {
			return 0, err
		}
		defer func() { _ = conn.Close(ctx) }()
		items, err := postgres.CollectExtensions(ctx, conn)
		if err != nil {
			return 0, err
		}
		if writer != nil && len(items) > 0 {
			envID, runID, err := runIDs(ctx)
			if err != nil {
				return 0, err
			}
			if err := writer.SaveExtendedObjectInventory(ctx, envID, runID, nil, nil, nil, items); err != nil {
				return 0, fmt.Errorf("persistir extensions: %w", err)
			}
		}
		return int64(len(items)), nil
	})

	must("timescale.version", func(ctx context.Context) (int64, error) {
		conn, err := connect(ctx)
		if err != nil {
			return 0, err
		}
		defer func() { _ = conn.Close(ctx) }()
		v, err := timescale.CollectVersion(ctx, conn)
		if err != nil {
			return 0, err
		}
		if v == nil {
			return 0, nil
		}
		if writer != nil {
			envID, runID, err := runIDs(ctx)
			if err != nil {
				return 0, err
			}
			status := timescale.StatusOK
			if !v.Compatible {
				status = timescale.StatusSkippedUnsupported
			}
			if err := writer.SaveTimescaleCoreInventory(ctx, envID, runID, timescale.InventoryResult{
				Status:  status,
				Version: v,
			}); err != nil {
				return 0, fmt.Errorf("persistir timescale version: %w", err)
			}
		}
		return 1, nil
	})

	must("timescale.hypertables", func(ctx context.Context) (int64, error) {
		conn, err := connect(ctx)
		if err != nil {
			return 0, err
		}
		defer func() { _ = conn.Close(ctx) }()
		items, err := timescale.CollectHypertables(ctx, conn, scope)
		if err != nil {
			return 0, err
		}
		if writer != nil && len(items) > 0 {
			envID, runID, err := runIDs(ctx)
			if err != nil {
				return 0, err
			}
			if err := writer.SaveTimescaleCoreInventory(ctx, envID, runID, timescale.InventoryResult{
				Status:      timescale.StatusOK,
				Hypertables: items,
			}); err != nil {
				return 0, fmt.Errorf("persistir hypertables: %w", err)
			}
		}
		return int64(len(items)), nil
	})

	must("timescale.dimensions", func(ctx context.Context) (int64, error) {
		conn, err := connect(ctx)
		if err != nil {
			return 0, err
		}
		defer func() { _ = conn.Close(ctx) }()
		items, err := timescale.CollectDimensions(ctx, conn, scope)
		if err != nil {
			return 0, err
		}
		if writer != nil && len(items) > 0 {
			envID, runID, err := runIDs(ctx)
			if err != nil {
				return 0, err
			}
			if err := writer.SaveTimescaleCoreInventory(ctx, envID, runID, timescale.InventoryResult{
				Status:     timescale.StatusOK,
				Dimensions: items,
			}); err != nil {
				return 0, fmt.Errorf("persistir dimensions: %w", err)
			}
		}
		return int64(len(items)), nil
	})

	must("timescale.chunks", func(ctx context.Context) (int64, error) {
		conn, err := connect(ctx)
		if err != nil {
			return 0, err
		}
		defer func() { _ = conn.Close(ctx) }()
		items, err := timescale.CollectChunks(ctx, conn, scope)
		if err != nil {
			return 0, err
		}
		if writer != nil && len(items) > 0 {
			envID, runID, err := runIDs(ctx)
			if err != nil {
				return 0, err
			}
			if err := writer.SaveTimescaleCoreInventory(ctx, envID, runID, timescale.InventoryResult{
				Status: timescale.StatusOK,
				Chunks: items,
			}); err != nil {
				return 0, fmt.Errorf("persistir chunks: %w", err)
			}
		}
		return int64(len(items)), nil
	})

	must("timescale.continuous_aggregates", func(ctx context.Context) (int64, error) {
		conn, err := connect(ctx)
		if err != nil {
			return 0, err
		}
		defer func() { _ = conn.Close(ctx) }()
		items, err := timescale.CollectContinuousAggregates(ctx, conn, scope)
		if err != nil {
			return 0, err
		}
		if writer != nil && len(items) > 0 {
			envID, runID, err := runIDs(ctx)
			if err != nil {
				return 0, err
			}
			if err := writer.SaveTimescalePolicyInventory(ctx, envID, runID, timescale.PolicyInventoryResult{
				Status:               timescale.StatusOK,
				ContinuousAggregates: items,
			}); err != nil {
				return 0, fmt.Errorf("persistir continuous aggregates: %w", err)
			}
		}
		return int64(len(items)), nil
	})

	must("timescale.jobs", func(ctx context.Context) (int64, error) {
		conn, err := connect(ctx)
		if err != nil {
			return 0, err
		}
		defer func() { _ = conn.Close(ctx) }()
		items, err := timescale.CollectJobs(ctx, conn, scope)
		if err != nil {
			return 0, err
		}
		if writer != nil && len(items) > 0 {
			envID, runID, err := runIDs(ctx)
			if err != nil {
				return 0, err
			}
			if err := writer.SaveTimescalePolicyInventory(ctx, envID, runID, timescale.PolicyInventoryResult{
				Status: timescale.StatusOK,
				Jobs:   items,
			}); err != nil {
				return 0, fmt.Errorf("persistir jobs: %w", err)
			}
		}
		return int64(len(items)), nil
	})

	must("timescale.policies", func(ctx context.Context) (int64, error) {
		conn, err := connect(ctx)
		if err != nil {
			return 0, err
		}
		defer func() { _ = conn.Close(ctx) }()
		items, err := timescale.CollectPolicies(ctx, conn, scope)
		if err != nil {
			return 0, err
		}
		if writer != nil && len(items) > 0 {
			envID, runID, err := runIDs(ctx)
			if err != nil {
				return 0, err
			}
			if err := writer.SaveTimescalePolicyInventory(ctx, envID, runID, timescale.PolicyInventoryResult{
				Status:   timescale.StatusOK,
				Policies: items,
			}); err != nil {
				return 0, fmt.Errorf("persistir policies: %w", err)
			}
		}
		return int64(len(items)), nil
	})

	return r
}

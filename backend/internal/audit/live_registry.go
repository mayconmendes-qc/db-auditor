package audit

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/mayconmendes-qc/timescale-auditor/internal/collectors/postgres"
	"github.com/mayconmendes-qc/timescale-auditor/internal/collectors/timescale"
	"github.com/mayconmendes-qc/timescale-auditor/internal/config"
)

// SnapshotSaver persists collected inventory for an audit run (optional).
type SnapshotSaver interface {
	SaveDiscovery(ctx context.Context, environmentID, auditRunID string, databases []postgres.DatabaseFacts, schemas []postgres.SchemaFacts) error
	SaveObjectInventory(ctx context.Context, environmentID, auditRunID string, tables []postgres.TableFacts, columns []postgres.ColumnFacts, indexes []postgres.IndexFacts) error
}

// LiveRegistryOptions configures collectors that hit audited databases.
type LiveRegistryOptions struct {
	Targets map[string]string
	Scope   config.Scope
	Saver   SnapshotSaver // optional; nil skips persistence
}

// NewLiveRegistry registers collectors that connect using AUDITOR_TARGET_DSN_* for the run environment.
func NewLiveRegistry(opts LiveRegistryOptions) *Registry {
	r := NewRegistry()
	scope := opts.Scope
	targets := opts.Targets
	if targets == nil {
		targets = config.LoadTargetDSNs()
	}

	connect := func(ctx context.Context) (*pgx.Conn, RunMeta, error) {
		meta, ok := RunMetaFromContext(ctx)
		if !ok || meta.EnvironmentID == "" {
			return nil, meta, fmt.Errorf("contexto da execução sem environment_id")
		}
		dsn := config.DSNForEnvironment(targets, meta.EnvironmentID)
		if dsn == "" {
			return nil, meta, fmt.Errorf(
				"DSN do ambiente não configurado. Defina %s no .env (credencial somente leitura) e reinicie a API",
				config.TargetDSNKey(meta.EnvironmentID),
			)
		}
		conn, err := pgx.Connect(ctx, dsn)
		if err != nil {
			return nil, meta, fmt.Errorf("conectar ao ambiente auditado: %w", err)
		}
		return conn, meta, nil
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
		conn, _, err := connect(ctx)
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
		conn, meta, err := connect(ctx)
		if err != nil {
			return 0, err
		}
		defer func() { _ = conn.Close(ctx) }()
		dbs, err := postgres.CollectDatabases(ctx, conn, scope)
		if err != nil {
			return 0, err
		}
		if opts.Saver != nil && meta.AuditRunID != "" {
			_ = opts.Saver.SaveDiscovery(ctx, meta.EnvironmentID, meta.AuditRunID, dbs, nil)
		}
		return int64(len(dbs)), nil
	})

	must("postgres.schemas", func(ctx context.Context) (int64, error) {
		conn, meta, err := connect(ctx)
		if err != nil {
			return 0, err
		}
		defer func() { _ = conn.Close(ctx) }()
		schemas, err := postgres.CollectSchemas(ctx, conn, scope)
		if err != nil {
			return 0, err
		}
		if opts.Saver != nil && meta.AuditRunID != "" {
			_ = opts.Saver.SaveDiscovery(ctx, meta.EnvironmentID, meta.AuditRunID, nil, schemas)
		}
		return int64(len(schemas)), nil
	})

	must("postgres.tables", func(ctx context.Context) (int64, error) {
		conn, meta, err := connect(ctx)
		if err != nil {
			return 0, err
		}
		defer func() { _ = conn.Close(ctx) }()
		tables, err := postgres.CollectTables(ctx, conn, scope)
		if err != nil {
			return 0, err
		}
		if opts.Saver != nil && meta.AuditRunID != "" {
			_ = opts.Saver.SaveObjectInventory(ctx, meta.EnvironmentID, meta.AuditRunID, tables, nil, nil)
		}
		return int64(len(tables)), nil
	})

	must("postgres.columns", func(ctx context.Context) (int64, error) {
		conn, meta, err := connect(ctx)
		if err != nil {
			return 0, err
		}
		defer func() { _ = conn.Close(ctx) }()
		cols, err := postgres.CollectColumns(ctx, conn, scope)
		if err != nil {
			return 0, err
		}
		if opts.Saver != nil && meta.AuditRunID != "" {
			_ = opts.Saver.SaveObjectInventory(ctx, meta.EnvironmentID, meta.AuditRunID, nil, cols, nil)
		}
		return int64(len(cols)), nil
	})

	must("postgres.indexes", func(ctx context.Context) (int64, error) {
		conn, meta, err := connect(ctx)
		if err != nil {
			return 0, err
		}
		defer func() { _ = conn.Close(ctx) }()
		idxs, err := postgres.CollectIndexes(ctx, conn, scope)
		if err != nil {
			return 0, err
		}
		if opts.Saver != nil && meta.AuditRunID != "" {
			_ = opts.Saver.SaveObjectInventory(ctx, meta.EnvironmentID, meta.AuditRunID, nil, nil, idxs)
		}
		return int64(len(idxs)), nil
	})

	must("postgres.constraints", func(ctx context.Context) (int64, error) {
		conn, _, err := connect(ctx)
		if err != nil {
			return 0, err
		}
		defer func() { _ = conn.Close(ctx) }()
		items, err := postgres.CollectConstraints(ctx, conn, scope)
		if err != nil {
			return 0, err
		}
		return int64(len(items)), nil
	})

	must("postgres.views", func(ctx context.Context) (int64, error) {
		conn, _, err := connect(ctx)
		if err != nil {
			return 0, err
		}
		defer func() { _ = conn.Close(ctx) }()
		items, err := postgres.CollectViews(ctx, conn, scope)
		if err != nil {
			return 0, err
		}
		return int64(len(items)), nil
	})

	must("postgres.functions", func(ctx context.Context) (int64, error) {
		conn, _, err := connect(ctx)
		if err != nil {
			return 0, err
		}
		defer func() { _ = conn.Close(ctx) }()
		items, err := postgres.CollectFunctions(ctx, conn, scope)
		if err != nil {
			return 0, err
		}
		return int64(len(items)), nil
	})

	must("postgres.extensions", func(ctx context.Context) (int64, error) {
		conn, _, err := connect(ctx)
		if err != nil {
			return 0, err
		}
		defer func() { _ = conn.Close(ctx) }()
		items, err := postgres.CollectExtensions(ctx, conn)
		if err != nil {
			return 0, err
		}
		return int64(len(items)), nil
	})

	must("timescale.version", func(ctx context.Context) (int64, error) {
		conn, _, err := connect(ctx)
		if err != nil {
			return 0, err
		}
		defer func() { _ = conn.Close(ctx) }()
		if _, err := timescale.CollectVersion(ctx, conn); err != nil {
			return 0, err
		}
		return 1, nil
	})

	must("timescale.hypertables", func(ctx context.Context) (int64, error) {
		conn, _, err := connect(ctx)
		if err != nil {
			return 0, err
		}
		defer func() { _ = conn.Close(ctx) }()
		items, err := timescale.CollectHypertables(ctx, conn)
		if err != nil {
			return 0, err
		}
		return int64(len(items)), nil
	})

	must("timescale.dimensions", func(ctx context.Context) (int64, error) {
		conn, _, err := connect(ctx)
		if err != nil {
			return 0, err
		}
		defer func() { _ = conn.Close(ctx) }()
		items, err := timescale.CollectDimensions(ctx, conn)
		if err != nil {
			return 0, err
		}
		return int64(len(items)), nil
	})

	must("timescale.chunks", func(ctx context.Context) (int64, error) {
		conn, _, err := connect(ctx)
		if err != nil {
			return 0, err
		}
		defer func() { _ = conn.Close(ctx) }()
		items, err := timescale.CollectChunks(ctx, conn)
		if err != nil {
			return 0, err
		}
		return int64(len(items)), nil
	})

	must("timescale.continuous_aggregates", func(ctx context.Context) (int64, error) {
		conn, _, err := connect(ctx)
		if err != nil {
			return 0, err
		}
		defer func() { _ = conn.Close(ctx) }()
		items, err := timescale.CollectContinuousAggregates(ctx, conn)
		if err != nil {
			return 0, err
		}
		return int64(len(items)), nil
	})

	must("timescale.jobs", func(ctx context.Context) (int64, error) {
		conn, _, err := connect(ctx)
		if err != nil {
			return 0, err
		}
		defer func() { _ = conn.Close(ctx) }()
		items, err := timescale.CollectJobs(ctx, conn)
		if err != nil {
			return 0, err
		}
		return int64(len(items)), nil
	})

	must("timescale.policies", func(ctx context.Context) (int64, error) {
		conn, _, err := connect(ctx)
		if err != nil {
			return 0, err
		}
		defer func() { _ = conn.Close(ctx) }()
		items, err := timescale.CollectPolicies(ctx, conn)
		if err != nil {
			return 0, err
		}
		return int64(len(items)), nil
	})

	return r
}

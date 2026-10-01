package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mayconmendes-qc/db-auditor/internal/analyzer"
	"github.com/mayconmendes-qc/db-auditor/internal/api"
	"github.com/mayconmendes-qc/db-auditor/internal/audit"
	"github.com/mayconmendes-qc/db-auditor/internal/config"
	"github.com/mayconmendes-qc/db-auditor/internal/database"
	"github.com/mayconmendes-qc/db-auditor/internal/observability"
	"github.com/mayconmendes-qc/db-auditor/internal/repository"
	"github.com/mayconmendes-qc/db-auditor/internal/scheduler"
)

func main() {
	observability.SetupLogging()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	targets := config.LoadTargetDSNs()
	if len(targets) == 0 {
		slog.Warn("nenhum AUDITOR_TARGET_DSN_* configurado; execuções de auditoria falharão até definir DSNs somente leitura")
	} else {
		slog.Info("target DSNs carregados", "count", len(targets))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.NewPool(ctx, cfg.Database)
	if err != nil {
		slog.Error("could not connect to snapshot store", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	store := repository.NewStore(pool)
	runStore := &repository.AuditRunStore{Store: store}
	liveOpts := audit.LiveRegistryOptions{
		Targets: targets,
		Scope:   cfg.Scope,
		Policy:  cfg.Collection,
		Writer:  store,
	}
	registry := audit.NewLiveRegistry(liveOpts)
	audit.AttachStructuralCollectors(registry, liveOpts)
	analysisService := analyzer.NewService(store, store, "1.0.0")
	runner := audit.NewRunner(registry, runStore, audit.RunnerOptions{
		ServiceVersion:    "0.14.0",
		CollectorVersion:  "1.1.0",
		MaxWorkers:        4,
		AnalysisProcessor: analysisService,
	})
	sch := scheduler.New(runner)

	server := &http.Server{
		Addr: cfg.HTTPAddress,
		Handler: api.NewHandlerWithOptions(store, api.HandlerOptions{
			Runner:   sch,
			Analysis: analysisService,
			Targets:  targets,
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				n := sch.TickDue(ctx)
				if n > 0 {
					slog.Info("scheduler triggered runs", "count", n)
				}
			}
		}
	}()

	go func() {
		slog.Info("HTTP server listening", "address", cfg.HTTPAddress)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server failed", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP server shutdown failed", "error", err)
	}
}

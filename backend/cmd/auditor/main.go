package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mayconmendes-qc/timescale-auditor/internal/api"
	"github.com/mayconmendes-qc/timescale-auditor/internal/config"
	"github.com/mayconmendes-qc/timescale-auditor/internal/database"
	"github.com/mayconmendes-qc/timescale-auditor/internal/repository"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
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

	server := &http.Server{
		Addr:              cfg.HTTPAddress,
		Handler:           api.NewHandler(store),
		ReadHeaderTimeout: 5 * time.Second,
	}

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

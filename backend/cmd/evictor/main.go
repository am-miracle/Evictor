package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/am-miracle/evictor/internal/api/handlers"
	"github.com/am-miracle/evictor/internal/api/services"
	"github.com/am-miracle/evictor/internal/app"
	"github.com/am-miracle/evictor/internal/config"
	"github.com/am-miracle/evictor/internal/logging"
	"github.com/am-miracle/evictor/internal/metrics"
	"github.com/am-miracle/evictor/internal/storage"
	"github.com/am-miracle/evictor/internal/workers"
)

const workerShutdownMargin = time.Second

func main() {
	os.Exit(run())
}

func run() int {
	cfg, err := config.FromEnvironment()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return 1
	}
	logger := logging.New(cfg.Environment, os.Stdout)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	shutdownBudget := app.NewShutdownBudget(ctx, cfg.ShutdownTimeout, workerShutdownMargin)

	store, err := storage.New(ctx, cfg.DatabaseURL.Reveal())
	if err != nil {
		logger.Error("database unavailable", "error", err)
		return 1
	}
	defer store.Close()

	role := cfg.Role
	if len(os.Args) > 1 {
		role, err = config.ParseRole(os.Args[1])
		if err != nil {
			logger.Error("invalid role argument", "error", err)
			return 1
		}
	}
	if role == config.RoleWorker {
		if err := workers.Run(ctx); err != nil {
			logger.Error("worker stopped", "error", err)
			return 1
		}
		return 0
	}
	counters := metrics.NewCounters(cfg.QueueCapacity)
	pool := workers.NewPool(workers.Options{
		Workers:      cfg.WorkerCount,
		Capacity:     cfg.QueueCapacity,
		MaxAttempts:  cfg.WorkerMaxAttempts,
		DrainTimeout: cfg.ShutdownTimeout,
		Logger:       logger,
		Metrics:      counters,
	})
	pool.Start(ctx)
	handler := handlers.NewRouter(handlers.Dependencies{
		Database: store.Pool(),
		Metrics:  counters,
		Logger:   logger,
	})
	listener, err := net.Listen("tcp", ":"+strconv.Itoa(cfg.Port))
	if err != nil {
		logger.Error("listen failed", "error", err)
		return 1
	}
	logger.Info("starting Evictor API", "address", listener.Addr().String())
	if err := services.Serve(ctx, listener, handler, cfg.ShutdownTimeout); err != nil {
		logger.Error("API stopped", "error", err)
		return 1
	}
	waitCtx, cancelWait := shutdownBudget.Context()
	defer cancelWait()
	if err := pool.Wait(waitCtx); err != nil {
		logger.Error("workers did not stop cleanly", "error", err)
		return 1
	}
	logger.Info("Evictor API stopped")
	return 0
}

package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"time"

	"github.com/am-miracle/evictor/internal/api/handlers"
	"github.com/am-miracle/evictor/internal/api/services"
	"github.com/am-miracle/evictor/internal/config"
	"github.com/am-miracle/evictor/internal/logging"
	"github.com/am-miracle/evictor/internal/metrics"
	"github.com/am-miracle/evictor/internal/workers"
)

const workerShutdownMargin = time.Second

type Database interface {
	Ping(context.Context) error
	Close()
}

type Drainer interface {
	Start(context.Context)
	Wait(context.Context) error
}

// Process owns the startup and shutdown sequence for every runtime role. Its
// ambient dependencies are injected so the sequence, and the exit code each
// outcome maps to, can be exercised without a database, a signal, or a port.
type Process struct {
	Lookup       config.Lookup
	Args         []string
	Output       io.Writer
	ErrorOutput  io.Writer
	Listen       func(network, address string) (net.Listener, error)
	OpenDatabase func(ctx context.Context, databaseURL string) (Database, error)
	NewWorkers   func(workers.Options) Drainer
}

// Run reports the process exit code. Cancelling ctx begins shutdown.
func (p Process) Run(ctx context.Context) int {
	cfg, err := config.Load(p.Lookup)
	if err != nil {
		_, _ = fmt.Fprintln(p.ErrorOutput, err)
		return 1
	}
	logger := logging.New(cfg.Environment, cfg.LogLevel, p.Output)
	slog.SetDefault(logger)

	budget := NewShutdownBudget(ctx, cfg.ShutdownTimeout, workerShutdownMargin)

	database, err := p.OpenDatabase(ctx, cfg.DatabaseURL.Reveal())
	if err != nil {
		logger.Error("database unavailable", "error", err)
		return 1
	}
	defer database.Close()

	role, err := p.role(cfg.Role)
	if err != nil {
		logger.Error("invalid role argument", "error", err)
		return 1
	}
	if role == config.RoleWorker {
		if err := workers.Run(ctx); err != nil {
			logger.Error("worker stopped", "error", err)
			return 1
		}
		return 0
	}
	return p.serveAPI(ctx, cfg, logger, database, budget)
}

func (p Process) role(configured config.Role) (config.Role, error) {
	if len(p.Args) > 1 {
		return config.ParseRole(p.Args[1])
	}
	return configured, nil
}

func (p Process) serveAPI(
	ctx context.Context,
	cfg config.Config,
	logger *slog.Logger,
	database Database,
	budget *ShutdownBudget,
) int {
	counters := metrics.NewCounters(cfg.QueueCapacity)
	pool := p.NewWorkers(workers.Options{
		Workers:      cfg.WorkerCount,
		Capacity:     cfg.QueueCapacity,
		MaxAttempts:  cfg.WorkerMaxAttempts,
		DrainTimeout: cfg.ShutdownTimeout,
		Logger:       logger,
		Metrics:      counters,
	})
	pool.Start(ctx)
	handler := handlers.NewRouter(handlers.Dependencies{
		Database: database,
		Metrics:  counters,
		Logger:   logger,
	})
	listener, err := p.Listen("tcp", ":"+strconv.Itoa(cfg.Port))
	if err != nil {
		logger.Error("listen failed", "error", err)
		return 1
	}
	logger.Info("starting Evictor API", "address", listener.Addr().String())
	if err := services.Serve(ctx, listener, handler, cfg.ShutdownTimeout); err != nil {
		logger.Error("API stopped", "error", err)
		return 1
	}
	waitCtx, cancelWait := budget.Context()
	defer cancelWait()
	switch err := pool.Wait(waitCtx); {
	case errors.Is(err, workers.ErrDrainIncomplete):
		// Shutdown was orderly and the pool already counted the loss, so this
		// surfaces on /metrics rather than as a failed container.
		logger.Error("workers abandoned at the drain deadline", "error", err)
	case err != nil:
		logger.Error("workers did not stop cleanly", "error", err)
		return 1
	}
	logger.Info("Evictor API stopped")
	return 0
}

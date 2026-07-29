// Package handlers contains HTTP route assembly and handlers for the versioned API.
package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/am-miracle/evictor/internal/api/middleware"
	"github.com/am-miracle/evictor/internal/metrics"
)

type DatabasePinger interface {
	Ping(context.Context) error
}

type Dependencies struct {
	Database DatabasePinger
	Metrics  *metrics.Counters
	Logger   *slog.Logger
}

func NewRouter(dependencies Dependencies) http.Handler {
	if dependencies.Logger == nil {
		dependencies.Logger = slog.Default()
	}
	if dependencies.Metrics == nil {
		dependencies.Metrics = metrics.NewCounters(0)
	}

	router := chi.NewRouter()
	router.Use(middleware.RequestLogger(dependencies.Logger))
	router.Get("/healthz", healthHandler)
	router.Get("/readyz", readinessHandler(dependencies.Database))
	router.Get("/metrics", metricsHandler(dependencies.Metrics))
	return router
}

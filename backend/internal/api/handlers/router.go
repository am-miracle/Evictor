// Package handlers contains HTTP handlers and middleware for the versioned API.
package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

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
	router.Use(requestLogger(dependencies.Logger))
	router.Get("/healthz", func(response http.ResponseWriter, _ *http.Request) {
		writeStatus(response, http.StatusOK, "ok")
	})
	router.Get("/readyz", func(response http.ResponseWriter, request *http.Request) {
		if dependencies.Database == nil || dependencies.Database.Ping(request.Context()) != nil {
			writeStatus(response, http.StatusServiceUnavailable, "not_ready")
			return
		}
		writeStatus(response, http.StatusOK, "ready")
	})
	router.Get("/metrics", metricsHandler(dependencies.Metrics))
	return router
}

func metricsHandler(counters *metrics.Counters) http.HandlerFunc {
	return func(response http.ResponseWriter, _ *http.Request) {
		snapshot := counters.Snapshot()
		response.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = fmt.Fprintf(response,
			"evictor_queue_depth %d\n"+
				"evictor_queue_capacity %d\n"+
				"evictor_jobs_processed_total %d\n"+
				"evictor_jobs_failed_total %d\n"+
				"evictor_jobs_dead_lettered_total %d\n"+
				"evictor_jobs_abandoned_total %d\n",
			snapshot.QueueDepth,
			snapshot.QueueCapacity,
			snapshot.JobsProcessed,
			snapshot.JobsFailed,
			snapshot.JobsDeadLettered,
			snapshot.JobsAbandoned,
		)
	}
}

func writeStatus(response http.ResponseWriter, code int, status string) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(code)
	_ = json.NewEncoder(response).Encode(map[string]string{"status": status})
}

type loggerContextKey struct{}

func Logger(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerContextKey{}).(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}

func requestLogger(base *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			// Correlation metadata is generated internally. Caller-controlled
			// values may contain credentials and must never become log fields.
			requestID := newRequestID()
			response.Header().Set("X-Request-ID", requestID)
			logger := base.With("request_id", requestID)
			request = request.WithContext(context.WithValue(request.Context(), loggerContextKey{}, logger))
			started := time.Now()
			next.ServeHTTP(response, request)
			logger.Info("request completed",
				"method", logSafeMethod(request.Method),
				"route", chi.RouteContext(request.Context()).RoutePattern(),
				"duration_ms", time.Since(started).Milliseconds(),
			)
		})
	}
}

func logSafeMethod(method string) string {
	switch method {
	case http.MethodConnect,
		http.MethodDelete,
		http.MethodGet,
		http.MethodHead,
		http.MethodOptions,
		http.MethodPatch,
		http.MethodPost,
		http.MethodPut,
		http.MethodTrace:
		return method
	default:
		return "OTHER"
	}
}

func newRequestID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(value[:])
}

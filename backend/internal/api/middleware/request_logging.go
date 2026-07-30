package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

type loggerContextKey struct{}

// Logger returns the request-scoped logger stored by RequestLogger. The logger
// includes the generated request ID for correlation.
func Logger(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerContextKey{}).(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}

func RequestLogger(base *slog.Logger) func(http.Handler) http.Handler {
	if base == nil {
		base = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			// Correlation metadata is generated internally. Caller-controlled
			// values may contain credentials and must never become log fields.
			requestID := newRequestID()
			response.Header().Set("X-Request-ID", requestID)
			ctx, state := withRequestContext(request.Context())
			logger := base.With("request_id", requestID)
			ctx = context.WithValue(ctx, loggerContextKey{}, logger)
			request = request.WithContext(ctx)
			wrapped := chimiddleware.NewWrapResponseWriter(response, request.ProtoMajor)
			started := time.Now()
			next.ServeHTTP(wrapped, request)

			status := wrapped.Status()
			if status == 0 {
				status = http.StatusOK
			}
			route := chi.RouteContext(request.Context()).RoutePattern()
			if isSuccessfulProbe(route, status) {
				return
			}

			attrs := []slog.Attr{
				slog.String("method", logSafeMethod(request.Method)),
				slog.String("route", route),
				slog.Int("status", status),
				slog.Int64("duration_ms", time.Since(started).Milliseconds()),
			}
			attrs = append(attrs, state.attributes()...)
			logger.LogAttrs(ctx, slog.LevelInfo, "request completed", attrs...)
		})
	}
}

func isSuccessfulProbe(route string, status int) bool {
	if status < http.StatusOK || status >= http.StatusBadRequest {
		return false
	}

	switch route {
	case "/healthz", "/readyz", "/metrics":
		return true
	default:
		return false
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

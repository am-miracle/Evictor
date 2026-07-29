package middleware_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/am-miracle/evictor/internal/api/middleware"
)

func TestRequestLoggerEmitsOneEnrichedCompletionLog(t *testing.T) {
	var logs bytes.Buffer
	router := chi.NewRouter()
	router.Use(middleware.RequestLogger(testLogger(&logs)))
	router.Post("/evictions/{evictionID}", func(response http.ResponseWriter, request *http.Request) {
		middleware.AddAttrs(request.Context(),
			slog.String("eviction_id", chi.URLParam(request, "evictionID")),
			slog.String("outcome", "accepted"),
		)
		response.WriteHeader(http.StatusAccepted)
	})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/evictions/ev_123", nil))

	logLine := logs.String()
	for _, want := range []string{
		`"msg":"request completed"`,
		`"method":"POST"`,
		`"route":"/evictions/{evictionID}"`,
		`"status":202`,
		`"eviction_id":"ev_123"`,
		`"outcome":"accepted"`,
	} {
		if !strings.Contains(logLine, want) {
			t.Errorf("completion log missing %s: %s", want, logLine)
		}
	}
	if lines := strings.Count(strings.TrimSpace(logLine), "\n") + 1; lines != 1 {
		t.Fatalf("got %d log lines, want 1: %s", lines, logLine)
	}
	if response.Header().Get("X-Request-ID") == "" {
		t.Fatal("missing X-Request-ID response header")
	}
}

func TestRequestLoggerProvidesCorrelatedLoggerToHandler(t *testing.T) {
	var logs bytes.Buffer
	router := chi.NewRouter()
	router.Use(middleware.RequestLogger(testLogger(&logs)))
	router.Get("/work", func(_ http.ResponseWriter, request *http.Request) {
		middleware.Logger(request.Context()).Info("operation failed")
	})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/work", nil))

	requestID := response.Header().Get("X-Request-ID")
	if occurrences := strings.Count(logs.String(), `"request_id":"`+requestID+`"`); occurrences != 2 {
		t.Fatalf("request ID should correlate handler and completion logs: %s", logs.String())
	}
}

func TestRequestLoggerSuppressesOnlySuccessfulProbeLogs(t *testing.T) {
	var logs bytes.Buffer
	router := chi.NewRouter()
	router.Use(middleware.RequestLogger(testLogger(&logs)))
	router.Get("/healthz", func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusOK)
	})
	router.Get("/readyz", func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusServiceUnavailable)
	})
	router.Get("/metrics", func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusOK)
	})

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/healthz", nil))
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/metrics", nil))
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/readyz", nil))

	logLine := logs.String()
	if strings.Contains(logLine, `"/healthz"`) || strings.Contains(logLine, `"/metrics"`) {
		t.Fatalf("successful probe was logged: %s", logLine)
	}
	if !strings.Contains(logLine, `"route":"/readyz"`) ||
		!strings.Contains(logLine, `"status":503`) {
		t.Fatalf("failed readiness probe was not logged: %s", logLine)
	}
}

func TestRequestLoggerBoundsConcurrentEnrichmentAndProtectsBaseFields(t *testing.T) {
	var logs bytes.Buffer
	router := chi.NewRouter()
	router.Use(middleware.RequestLogger(testLogger(&logs)))
	router.Get("/work", func(_ http.ResponseWriter, request *http.Request) {
		var group sync.WaitGroup
		for index := range 32 {
			group.Add(1)
			go func() {
				defer group.Done()
				middleware.AddAttrs(request.Context(), slog.Int("worker", index))
			}()
		}
		group.Wait()
	})

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/work", nil))

	logLine := logs.String()
	if got := strings.Count(logLine, `"worker":`); got != middleware.MaxRequestAttrs {
		t.Fatalf("got %d enriched attrs, want bounded maximum %d: %s",
			got, middleware.MaxRequestAttrs, logLine)
	}
}

func TestRequestLoggerRejectsProtectedAndOpaqueEnrichment(t *testing.T) {
	const credentialCanary = "credential_material_do_not_log"
	var logs bytes.Buffer
	router := chi.NewRouter()
	router.Use(middleware.RequestLogger(testLogger(&logs)))
	router.Get("/work", func(_ http.ResponseWriter, request *http.Request) {
		middleware.AddAttrs(request.Context(),
			slog.String("request_id", credentialCanary),
			slog.String("API-Key", credentialCanary),
			slog.Group("metadata", slog.String("authorization", credentialCanary)),
			slog.Any("payload", map[string]string{"token": credentialCanary}),
			slog.String("outcome", "accepted"),
		)
	})

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/work", nil))

	logLine := logs.String()
	if strings.Contains(logLine, credentialCanary) {
		t.Fatalf("credential-bearing enrichment appeared in log: %s", logLine)
	}
	if !strings.Contains(logLine, `"outcome":"accepted"`) {
		t.Fatalf("safe enrichment was discarded: %s", logLine)
	}
}

func TestRequestLoggerDoesNotLogCallerControlledCredentialMaterial(t *testing.T) {
	const credentialCanary = "evictor_test_api_key_do_not_log"
	var logs bytes.Buffer
	router := chi.NewRouter()
	router.Use(middleware.RequestLogger(testLogger(&logs)))
	router.Get("/safe", func(_ http.ResponseWriter, _ *http.Request) {})
	request := httptest.NewRequest(http.MethodGet, "/safe", nil)
	request.Method = credentialCanary
	request.Header.Set("X-Request-ID", credentialCanary)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if strings.Contains(logs.String(), credentialCanary) {
		t.Fatalf("caller-controlled credential material appeared in logs: %s", logs.String())
	}
	if response.Header().Get("X-Request-ID") == credentialCanary {
		t.Fatal("caller-controlled request ID was trusted")
	}
	if !strings.Contains(logs.String(), `"method":"OTHER"`) {
		t.Fatalf("extension method was not normalized: %s", logs.String())
	}
}

func testLogger(destination *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(destination, nil))
}

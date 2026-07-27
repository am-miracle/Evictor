package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/am-miracle/evictor/internal/api/handlers"
	"github.com/am-miracle/evictor/internal/metrics"
)

type pinger struct{ err error }

func (p pinger) Ping(context.Context) error { return p.err }

func TestHealthzReturnsJSON(t *testing.T) {
	response := serve(t, handlers.Dependencies{}, "/healthz")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	assertStatusBody(t, response, "ok")
}

func TestReadyzReflectsDatabasePing(t *testing.T) {
	t.Run("ready", func(t *testing.T) {
		response := serve(t, handlers.Dependencies{Database: pinger{}}, "/readyz")
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d", response.Code)
		}
		assertStatusBody(t, response, "ready")
	})
	t.Run("not ready", func(t *testing.T) {
		response := serve(t, handlers.Dependencies{Database: pinger{err: errors.New("offline")}}, "/readyz")
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d", response.Code)
		}
		assertStatusBody(t, response, "not_ready")
	})
}

func TestMetricsExposesWorkerCounters(t *testing.T) {
	counters := metrics.NewCounters(10)
	counters.SetQueueDepth(4)
	counters.JobProcessed()
	response := serve(t, handlers.Dependencies{Metrics: counters}, "/metrics")
	if got := response.Body.String(); !strings.Contains(got, "evictor_queue_depth 4\n") ||
		!strings.Contains(got, "evictor_jobs_processed_total 1\n") {
		t.Fatalf("unexpected metrics:\n%s", got)
	}
}

func TestRequestMiddlewareProvidesAndLogsRequestID(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	response := serve(t, handlers.Dependencies{Logger: logger}, "/healthz")
	requestID := response.Header().Get("X-Request-ID")
	if requestID == "" {
		t.Fatal("missing X-Request-ID")
	}
	if !strings.Contains(logs.String(), requestID) {
		t.Fatalf("request ID %q missing from logs: %s", requestID, logs.String())
	}
}

func TestRequestMiddlewareDoesNotLogCallerControlledCredentialMaterial(t *testing.T) {
	const credentialCanary = "evictor_test_api_key_do_not_log"
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	request := httptest.NewRequest(http.MethodGet, "/"+credentialCanary, nil)
	request.Method = credentialCanary
	request.Header.Set("X-Request-ID", credentialCanary)
	response := httptest.NewRecorder()

	handlers.NewRouter(handlers.Dependencies{Logger: logger}).ServeHTTP(response, request)

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

func serve(t *testing.T, dependencies handlers.Dependencies, path string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	response := httptest.NewRecorder()
	handlers.NewRouter(dependencies).ServeHTTP(response, request)
	return response
}

func assertStatusBody(t *testing.T, response *httptest.ResponseRecorder, want string) {
	t.Helper()
	if got := response.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("content type = %q", got)
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Status != want {
		t.Fatalf("status body = %q, want %q", body.Status, want)
	}
}

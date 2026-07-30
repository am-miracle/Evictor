package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
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
	counters.AddJobsAbandoned(3)
	response := serve(t, handlers.Dependencies{Metrics: counters}, "/metrics")
	if got := response.Body.String(); !strings.Contains(got, "evictor_queue_depth 4\n") ||
		!strings.Contains(got, "evictor_jobs_processed_total 1\n") ||
		!strings.Contains(got, "evictor_jobs_abandoned_total 3\n") {
		t.Fatalf("unexpected metrics:\n%s", got)
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

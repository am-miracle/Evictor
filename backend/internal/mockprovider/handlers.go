package mockprovider

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type statusResponse struct {
	ID          string      `json:"id"`
	WorkerState WorkerState `json:"worker_state"`
	WorkerCount int         `json:"worker_count"`
	WorkersMin  int         `json:"workers_min"`
	AsOf        time.Time   `json:"as_of"`
}

type invokeResponse struct {
	WasColdStart      bool   `json:"was_cold_start"`
	LatencyMs         int    `json:"latency_ms"`
	ProviderRequestID string `json:"provider_request_id"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func handleGetStatus(store *Store, clock Clock, sleeper Sleeper) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		now := clock.Now()

		scenario := store.Get(id, now).Scenario
		scenario.delay(sleeper)

		// Meaning that a failure applied here and should therefore return
		if scenario.applyFailureHeader(now, w) {
			return
		}

		ep := store.Status(id, now)

		writeJSON(w, http.StatusOK, statusResponse{
			ID:          ep.ID,
			WorkerState: ep.WorkerState,
			WorkerCount: ep.WorkerCount,
			WorkersMin:  ep.WorkersMin,
			AsOf:        now,
		})
	}
}

func handleInvoke(store *Store, clock Clock, sleeper Sleeper) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		now := clock.Now()

		scenario := store.Get(id, now).Scenario
		scenario.delay(sleeper)
		if scenario.applyFailureHeader(now, w) {
			return
		}

		ep := store.Invoke(id, now)
		wasCold := ep.WorkerState != WarmState
		latency := warmLatencyMs

		if wasCold {
			latency = int(ColdStartDuration.Milliseconds())
		}

		writeJSON(w, http.StatusOK, invokeResponse{
			WasColdStart:      wasCold,
			LatencyMs:         latency,
			ProviderRequestID: newRequestID(),
		})
	}
}

const warmLatencyMs = 120

func newRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("mock-%x", b)
}

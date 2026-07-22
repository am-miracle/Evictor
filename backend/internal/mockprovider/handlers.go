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

func handleGetStatus(store *Store, clock Clock) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		now := clock.Now()

		endPointState := store.Get(id, now)
		endPointState.Scenario.delay()
		// Meaning that a failure applied here and should therefore return
		if endPointState.Scenario.applyFailureHeader(now, w) {
			return
		}
		advance(endPointState, now)

		writeJSON(w, http.StatusOK, statusResponse{
			ID:          endPointState.ID,
			WorkerState: endPointState.WorkerState,
			WorkerCount: endPointState.WorkerCount,
			WorkersMin:  endPointState.WorkersMin,
			AsOf:        now,
		})
	}
}

func handleInvoke(store *Store, clock Clock) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		now := clock.Now()

		ep := store.Get(id, now)
		ep.Scenario.delay()
		if ep.Scenario.applyFailureHeader(now, w) {
			return
		}

		advance(ep, now) // resolve any pending time-based transition first

		wasCold := ep.WorkerState != WarmState
		if ep.WorkerState == ColdState {
			ep.WorkerState = WarmingState
			ep.LastTransitionAt = now
		}
		ep.LastInvokeAt = now

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

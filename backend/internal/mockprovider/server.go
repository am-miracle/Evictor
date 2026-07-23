package mockprovider

import (
	"net/http"
	"time"
)

const ReadHeaderTimeout = 5 * time.Second

// NewServer wires the mock provider's routes onto a mux, given the
// state store and clock the caller has already constructed. sleeper
// governs how Scenario.LatencyMs is honored; pass RealSleeper{} when
// actually running the service.
func NewServer(store *Store, clock Clock, sleeper Sleeper) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", healthz)
	mux.HandleFunc("GET /v1/endpoints/{id}", handleGetStatus(store, clock, sleeper))
	mux.HandleFunc("POST /v1/endpoints/{id}/invoke", handleInvoke(store, clock, sleeper))

	return mux
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

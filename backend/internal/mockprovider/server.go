package mockprovider

import (
	"encoding/json"
	"net/http"
	"time"
)

const ReadHeaderTimeout = 5 * time.Second

// NewServer wires the mock provider's routes onto a mux, given the
// state store and clock the caller has already constructed.
func NewServer(store *Store, clock Clock) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", healthz)
	mux.HandleFunc("GET /v1/endpoints/{id}", handleGetStatus(store, clock))
	mux.HandleFunc("POST /v1/endpoints/{id}/invoke", handleInvoke(store, clock))

	return mux
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

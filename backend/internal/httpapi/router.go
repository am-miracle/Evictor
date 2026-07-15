package httpapi

import (
	"encoding/json"
	"net/http"
	"time"
)

const ReadHeaderTimeout = 5 * time.Second

func NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz)
	return mux
}

func healthz(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(response).Encode(map[string]string{"status": "ok"})
}

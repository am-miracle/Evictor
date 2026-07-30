package handlers

import (
	"encoding/json"
	"net/http"
)

func healthHandler(response http.ResponseWriter, _ *http.Request) {
	writeProbeStatus(response, http.StatusOK, "ok")
}

func readinessHandler(database DatabasePinger) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if database == nil || database.Ping(request.Context()) != nil {
			writeProbeStatus(response, http.StatusServiceUnavailable, "not_ready")
			return
		}
		writeProbeStatus(response, http.StatusOK, "ready")
	}
}

func writeProbeStatus(response http.ResponseWriter, code int, status string) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(code)
	_ = json.NewEncoder(response).Encode(map[string]string{"status": status})
}

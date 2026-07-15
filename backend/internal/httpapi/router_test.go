package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestContractConventions_HealthzReturnsJSON(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()

	NewHandler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("content type = %q", got)
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "ok" {
		t.Fatalf("status body = %q", body.Status)
	}
}

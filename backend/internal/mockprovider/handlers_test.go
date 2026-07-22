package mockprovider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newInvokeRequest(id string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/v1/endpoints/"+id+"/invoke", nil)
	r.SetPathValue("id", id)
	return r
}

func newStatusRequest(id string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/v1/endpoints/"+id, nil)
	r.SetPathValue("id", id)
	return r
}

func TestHandleInvoke_ColdStartOnFirstInvoke(t *testing.T) {
	store := NewStore()
	clock := NewVirtualClock(time.Now())
	handler := handleInvoke(store, clock)

	rec := httptest.NewRecorder()
	handler(rec, newInvokeRequest("ep_demo"))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body invokeResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !body.WasColdStart {
		t.Fatalf("expected first invoke on an unseen endpoint to be a cold start")
	}
	if body.LatencyMs != int(ColdStartDuration.Milliseconds()) {
		t.Fatalf("expected latency_ms %d, got %d", int(ColdStartDuration.Milliseconds()), body.LatencyMs)
	}
}

func TestHandleInvoke_StillColdImmediatelyAfterFirstInvoke(t *testing.T) {
	store := NewStore()
	clock := NewVirtualClock(time.Now())
	handler := handleInvoke(store, clock)

	handler(httptest.NewRecorder(), newInvokeRequest("ep_demo"))

	rec := httptest.NewRecorder()
	handler(rec, newInvokeRequest("ep_demo"))

	var body invokeResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !body.WasColdStart {
		t.Fatalf("expected an invoke arriving while still warming up (no time has passed) to still report a cold start, got was_cold_start=false with latency_ms=%d", body.LatencyMs)
	}
}

func TestHandleInvoke_WarmAfterColdStartDurationPasses(t *testing.T) {
	store := NewStore()
	clock := NewVirtualClock(time.Now())
	handler := handleInvoke(store, clock)

	handler(httptest.NewRecorder(), newInvokeRequest("ep_demo"))

	clock.Advance(ColdStartDuration + time.Second)

	rec := httptest.NewRecorder()
	handler(rec, newInvokeRequest("ep_demo"))

	var body invokeResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.WasColdStart {
		t.Fatalf("expected invoke after the endpoint had time to warm up to be a warm start")
	}
	if body.LatencyMs != warmLatencyMs {
		t.Fatalf("expected warm latency_ms %d, got %d", warmLatencyMs, body.LatencyMs)
	}
}

func TestHandleGetStatus_RateLimitedScenario(t *testing.T) {
	store := NewStore()
	clock := NewVirtualClock(time.Now())
	handler := handleGetStatus(store, clock)

	store.SetScenario("ep_demo", Scenario{
		Failure:      FailureRateLimited,
		FailureUntil: clock.Now().Add(30 * time.Second),
	}, clock.Now())

	rec := httptest.NewRecorder()
	handler(rec, newStatusRequest("ep_demo"))

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatalf("expected a Retry-After header on a rate-limited response")
	}
}

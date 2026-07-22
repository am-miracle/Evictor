package mockprovider

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestScenario_DelaySleepsForConfiguredLatency(t *testing.T) {
	store := NewStore()
	clock := NewVirtualClock(time.Now())

	store.SetScenario("ep_demo", Scenario{LatencyMs: 30}, clock.Now())

	handler := handleGetStatus(store, clock)

	start := time.Now()
	handler(httptest.NewRecorder(), newStatusRequest("ep_demo"))
	elapsed := time.Since(start)

	if elapsed < 30*time.Millisecond {
		t.Fatalf("expected the handler to take at least 30ms due to scenario latency, took %v", elapsed)
	}
}

func TestScenario_NoDelayWhenLatencyMsIsZero(t *testing.T) {
	store := NewStore()
	clock := NewVirtualClock(time.Now())
	handler := handleGetStatus(store, clock)

	start := time.Now()
	handler(httptest.NewRecorder(), newStatusRequest("ep_demo"))
	elapsed := time.Since(start)

	if elapsed > 20*time.Millisecond {
		t.Fatalf("expected no artificial delay with a default scenario, took %v", elapsed)
	}
}

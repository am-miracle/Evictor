package mockprovider

import (
	"net/http/httptest"
	"testing"
	"time"
)

// recordingSleeper stands in for RealSleeper in tests: instead of
// actually waiting, it just records what it was asked to sleep for, so
// tests can assert on that directly rather than measuring wall-clock
// time (which is flaky on slow or -race-instrumented runners).
type recordingSleeper struct {
	durations []time.Duration
}

func (s *recordingSleeper) Sleep(d time.Duration) {
	s.durations = append(s.durations, d)
}

func TestScenario_DelaySleepsForConfiguredLatency(t *testing.T) {
	store := NewStore()
	clock := NewVirtualClock(time.Now())
	sleeper := &recordingSleeper{}

	store.SetScenario("ep_demo", Scenario{LatencyMs: 30}, clock.Now())

	handler := handleGetStatus(store, clock, sleeper)
	handler(httptest.NewRecorder(), newStatusRequest("ep_demo"))

	if len(sleeper.durations) != 1 {
		t.Fatalf("expected exactly one sleep call, got %d", len(sleeper.durations))
	}
	if sleeper.durations[0] != 30*time.Millisecond {
		t.Fatalf("expected a 30ms sleep, got %v", sleeper.durations[0])
	}
}

func TestScenario_NoDelayWhenLatencyMsIsZero(t *testing.T) {
	store := NewStore()
	clock := NewVirtualClock(time.Now())
	sleeper := &recordingSleeper{}

	handler := handleGetStatus(store, clock, sleeper)
	handler(httptest.NewRecorder(), newStatusRequest("ep_demo"))

	if len(sleeper.durations) != 0 {
		t.Fatalf("expected no sleep with a default scenario, got %v", sleeper.durations)
	}
}

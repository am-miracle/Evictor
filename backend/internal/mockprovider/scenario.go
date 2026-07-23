package mockprovider

import (
	"net/http"
	"time"
)

type FailureMode string

const (
	FailureNone        FailureMode = "none"
	FailureRateLimited FailureMode = "rate_limited"
	FailureServerError FailureMode = "server_error"
)

// Scenario is the currently-active chaos configuration for one endpoint.
type Scenario struct {
	LatencyMs    int         // artificial delay added before responding
	Failure      FailureMode // what kind of failure to simulate, if any
	FailureUntil time.Time   // the failure clears once Now() passes this
}

// DefaultScenario is what a brand-new endpoint gets: no chaos at all.
func DefaultScenario() Scenario {
	return Scenario{Failure: FailureNone}
}

// Sleeper abstracts "actually wait this long," so tests can assert on
// what delay would have happened without paying for it in wall-clock time.
type Sleeper interface {
	Sleep(d time.Duration)
}

// RealSleeper is what the running service uses: an actual time.Sleep.
type RealSleeper struct{}

func (RealSleeper) Sleep(d time.Duration) {
	time.Sleep(d)
}

// delay tells sleeper to wait for the scenario's configured artificial
// latency, if any. This simulates how long the caller's own request
// actually takes to come back, not a change in the mock provider's
// simulated notion of time, so it's independent of which Clock is in use.
func (s Scenario) delay(sleeper Sleeper) {
	if s.LatencyMs > 0 {
		sleeper.Sleep(time.Duration(s.LatencyMs) * time.Millisecond)
	}
}

// applyFailureHeader checks whether the scenario says to fail this request right now.
// If it does, it writes the failure response and returns true so the
// caller (which is a handler) knows to stop and not process the request further.
func (s Scenario) applyFailureHeader(now time.Time, w http.ResponseWriter) bool {
	if s.Failure != FailureNone && now.Before(s.FailureUntil) {
		switch s.Failure {
		case FailureRateLimited:
			w.Header().Set("Retry-After", "5")
			w.WriteHeader(http.StatusTooManyRequests)
		case FailureServerError:
			w.WriteHeader(http.StatusInternalServerError)
		}
		return true
	}
	return false
}

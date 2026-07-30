package mockprovider

import (
	"sync"
	"time"
)

type WorkerState string

const (
	ColdState    WorkerState = "cold"
	WarmingState WorkerState = "warming"
	WarmState    WorkerState = "warm"
)

const (
	ColdStartDuration = 8 * time.Second // how long "warming" lasts before becoming "warm"
	IdleTimeout       = 5 * time.Minute // how long "warm" persists with no action before reverting to cold
)

type EndpointState struct {
	ID               string
	WorkerState      WorkerState
	WorkerCount      int       // This tracks how many fake GPUs are on
	WorkersMin       int       // Mirrors RunPod's real workersMin field: keep at least this many workers on
	LastTransitionAt time.Time // Keeps track of when last its state changed
	LastInvokeAt     time.Time // Tells when the GPU was actually used
	Scenario         Scenario  // Active chaos/fault-injection config for this endpoint
}

type Store struct {
	mu        sync.Mutex
	endpoints map[string]*EndpointState
}

func NewStore() *Store {
	return &Store{
		endpoints: make(map[string]*EndpointState),
	}
}

func (s *Store) Get(id string, now time.Time) EndpointState {
	s.mu.Lock()
	defer s.mu.Unlock()

	return *s.getOrCreateLocked(id, now)
}

// SetScenario replaces the active chaos config for an endpoint, creating
// the endpoint first if it hasn't been seen yet.
func (s *Store) SetScenario(id string, scenario Scenario, now time.Time) {
	s.Update(id, now, func(es *EndpointState) {
		es.Scenario = scenario
	})
}

func (s *Store) Status(id string, now time.Time) EndpointState {
	return s.Update(id, now, func(ep *EndpointState) {
		advance(ep, now)
	})
}

func (s *Store) Invoke(id string, now time.Time) EndpointState {
	return s.Update(id, now, func(ep *EndpointState) {
		advance(ep, now)
		if ep.WorkerState == ColdState {
			ep.WorkerState = WarmingState
			ep.LastTransitionAt = now
		}
		ep.LastInvokeAt = now
	})
}

// Update runs fn against the endpoint's live state while the store's lock
// is held, then returns a snapshot. This is the only place mutation happens.
func (s *Store) Update(id string, now time.Time, fn func(*EndpointState)) EndpointState {
	s.mu.Lock()
	defer s.mu.Unlock()

	ep := s.getOrCreateLocked(id, now)
	fn(ep)

	return *ep
}

// getOrCreateLocked assumes s.mu is already held by the caller.
func (s *Store) getOrCreateLocked(id string, now time.Time) *EndpointState {
	endPointState, ok := s.endpoints[id]
	if !ok {
		endPointState = &EndpointState{
			ID:               id,
			WorkerState:      ColdState,
			LastTransitionAt: now,
			Scenario:         DefaultScenario(),
		}
		s.endpoints[id] = endPointState
	}
	return endPointState
}

func (s *Store) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.endpoints = make(map[string]*EndpointState)
}

// advance applies every time-based state transition implied by now, not
// just one — if now has jumped far enough to cross multiple thresholds
// (e.g. past both ColdStartDuration and IdleTimeout), it keeps resolving
// until the state matches what should actually be true at now. It does
// nothing to Cold endpoints — only an invoke or a WorkersMin bump can
// wake one up.
func advance(ep *EndpointState, now time.Time) {
	for {
		switch ep.WorkerState {
		case WarmingState:
			warmedAt := ep.LastTransitionAt.Add(ColdStartDuration)
			if now.Before(warmedAt) {
				return
			}
			ep.WorkerState = WarmState
			ep.WorkerCount = 1
			ep.LastTransitionAt = warmedAt

		case WarmState:
			idleAt := ep.LastInvokeAt.Add(IdleTimeout)
			if ep.WorkersMin > 0 || now.Before(idleAt) {
				return
			}
			ep.WorkerState = ColdState
			ep.WorkerCount = 0
			ep.LastTransitionAt = idleAt

		default:
			return
		}
	}
}

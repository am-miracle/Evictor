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

func (s *Store) Get(id string, now time.Time) *EndpointState {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.getOrCreateLocked(id, now)
}

// SetScenario replaces the active chaos config for an endpoint, creating
// the endpoint first if it hasn't been seen yet.
func (s *Store) SetScenario(id string, scenario Scenario, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.getOrCreateLocked(id, now).Scenario = scenario
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

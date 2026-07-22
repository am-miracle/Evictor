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

	endPointState, ok := s.endpoints[id]
	if !ok {
		endPointState = &EndpointState{
			ID:               id,
			WorkerState:      ColdState,
			LastTransitionAt: now,
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

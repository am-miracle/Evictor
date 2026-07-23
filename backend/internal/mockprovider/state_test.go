package mockprovider

import (
	"sync"
	"testing"
	"time"
)

func TestStore_InvokeMutationPersistsThroughGet(t *testing.T) {
	store := NewStore()
	now := time.Now()

	store.Invoke("ep_demo", now)

	again := store.Get("ep_demo", now)
	if again.WorkerState != WarmingState {
		t.Fatalf("expected the invoke's state change to persist, got %q", again.WorkerState)
	}
}

func TestStore_GetDoesNotMutateState(t *testing.T) {
	store := NewStore()
	now := time.Now()

	first := store.Get("ep_demo", now)
	first.WorkerState = WarmState // mutating the returned copy, not the store

	again := store.Get("ep_demo", now)
	if again.WorkerState != ColdState {
		t.Fatalf("expected Get to return an independent copy that mutating can't affect the store, got %q", again.WorkerState)
	}
}

func TestStore_GetCreatesColdStateForUnseenID(t *testing.T) {
	store := NewStore()
	now := time.Now()

	ep := store.Get("ep_unseen", now)

	if ep.WorkerState != ColdState {
		t.Fatalf("expected a fresh endpoint to start %q, got %q", ColdState, ep.WorkerState)
	}
	if !ep.LastTransitionAt.Equal(now) {
		t.Fatalf("expected LastTransitionAt to equal %v, got %v", now, ep.LastTransitionAt)
	}
}

func TestStore_Reset(t *testing.T) {
	store := NewStore()
	now := time.Now()

	store.Invoke("ep_demo", now)

	store.Reset()

	again := store.Get("ep_demo", now)
	if again.WorkerState != ColdState {
		t.Fatalf("expected Reset to clear state back to %q, got %q", ColdState, again.WorkerState)
	}
}

// TestStore_ConcurrentInvokesDoNotRace fires many goroutines at the same
// endpoint ID simultaneously. Run with -race: before the Update-based
// redesign, Store.Get handed out a live pointer that handlers mutated
// completely unsynchronized, which this test would have caught.
func TestStore_ConcurrentInvokesDoNotRace(t *testing.T) {
	store := NewStore()
	now := time.Now()

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			store.Invoke("ep_demo", now)
			store.Get("ep_demo", now)
			store.SetScenario("ep_demo", DefaultScenario(), now)
		}()
	}

	wg.Wait()

	ep := store.Get("ep_demo", now)
	if ep.WorkerState != WarmingState {
		t.Fatalf("expected concurrent invokes on a cold endpoint to settle on %q, got %q", WarmingState, ep.WorkerState)
	}
}

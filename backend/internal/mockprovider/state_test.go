package mockprovider

import (
	"testing"
	"time"
)

func TestStore_GetReturnsSameCardOnSecondCall(t *testing.T) {
	store := NewStore()
	now := time.Now()

	first := store.Get("ep_demo", now)
	second := store.Get("ep_demo", now)

	if first != second {
		t.Fatalf("expected Get to return the same *EndpointState, got two different pointers")
	}
}

func TestStore_GetMutationPersists(t *testing.T) {
	store := NewStore()
	now := time.Now()

	ep := store.Get("ep_demo", now)
	ep.WorkerState = WarmState

	again := store.Get("ep_demo", now)

	if again.WorkerState != WarmState {
		t.Fatalf("expected mutated state %q to persist, got %q", WarmState, again.WorkerState)
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

	ep := store.Get("ep_demo", now)
	ep.WorkerState = WarmState

	store.Reset()

	again := store.Get("ep_demo", now)
	if again.WorkerState != ColdState {
		t.Fatalf("expected Reset to clear state back to %q, got %q", ColdState, again.WorkerState)
	}
}

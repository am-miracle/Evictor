package app

import (
	"context"
	"testing"
	"time"
)

func TestShutdownBudgetDoesNotBlockBeforeShutdown(t *testing.T) {
	now := time.Now()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	budget := newShutdownBudget(
		ctx,
		time.Minute,
		time.Second,
		func() time.Time { return now },
	)

	if got, want := budget.deadline(), now.Add(time.Minute+time.Second); !got.Equal(want) {
		t.Fatalf("deadline = %v, want %v", got, want)
	}
}

func TestShutdownBudgetAnchorsToCancellationObservation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	observedAt := time.Now().Add(-9 * time.Second)
	budget := newShutdownBudget(
		ctx,
		15*time.Second,
		time.Second,
		func() time.Time { return observedAt },
	)

	if got, want := budget.deadline(), observedAt.Add(16*time.Second); !got.Equal(want) {
		t.Fatalf("deadline = %v, want %v", got, want)
	}
}

func TestShutdownBudgetAwaitsLateCancellationObserver(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	observedAt := time.Now().Add(-9 * time.Second)
	releaseClock := make(chan struct{})
	budget := newShutdownBudget(
		ctx,
		15*time.Second,
		time.Second,
		func() time.Time {
			<-releaseClock
			return observedAt
		},
	)

	result := make(chan time.Time, 1)
	go func() { result <- budget.deadline() }()
	select {
	case <-result:
		t.Fatal("deadline returned before cancellation observation completed")
	case <-time.After(20 * time.Millisecond):
	}
	close(releaseClock)

	if got, want := <-result, observedAt.Add(16*time.Second); !got.Equal(want) {
		t.Fatalf("deadline = %v, want %v", got, want)
	}
}

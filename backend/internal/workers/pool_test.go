package workers_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/am-miracle/evictor/internal/metrics"
	"github.com/am-miracle/evictor/internal/workers"
)

func TestSubmitReturnsErrQueueFullWithoutBlocking_BR14(t *testing.T) {
	pool := workers.NewPool(workers.Options{Workers: 1, Capacity: 1})
	block := make(chan struct{})
	if err := pool.Submit(func(context.Context) error { <-block; return nil }); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	err := pool.Submit(func(context.Context) error { return nil })
	if !errors.Is(err, workers.ErrQueueFull) {
		t.Fatalf("got %v, want ErrQueueFull", err)
	}
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Fatalf("submit blocked for %v", elapsed)
	}
	close(block)
}

func TestQueueDepthTracksInterleavedSubmissionsAndDequeues(t *testing.T) {
	counters := metrics.NewCounters(64)
	pool := workers.NewPool(workers.Options{
		Workers:  1,
		Capacity: 64,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Metrics:  counters,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool.Start(ctx)

	block := make(chan struct{})
	started := make(chan struct{})
	if err := pool.Submit(func(context.Context) error {
		close(started)
		<-block
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	<-started

	const queued = 50
	for range queued {
		if err := pool.Submit(func(context.Context) error { return nil }); err != nil {
			t.Fatal(err)
		}
	}
	if got := counters.Snapshot().QueueDepth; got != queued {
		t.Fatalf("queue depth = %d, want %d", got, queued)
	}

	close(block)
	for deadline := time.Now().Add(time.Second); counters.Snapshot().QueueDepth != 0 && time.Now().Before(deadline); {
		time.Sleep(time.Millisecond)
	}
	if got := counters.Snapshot().QueueDepth; got != 0 {
		t.Fatalf("queue depth after processing = %d, want 0", got)
	}
}

func TestCancellationStopsWorkersWithinDeadline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var processed atomic.Int64
	started := make(chan struct{})
	executionContext := make(chan context.Context, 1)
	release := make(chan struct{})
	drainedWithLiveContext := make(chan bool, 1)
	pool := workers.NewPool(workers.Options{Workers: 1, Capacity: 2})
	if err := pool.Submit(func(ctx context.Context) error {
		close(started)
		executionContext <- ctx
		<-release
		processed.Add(1)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := pool.Submit(func(ctx context.Context) error {
		drainedWithLiveContext <- ctx.Err() == nil
		processed.Add(1)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	pool.Start(ctx)
	<-started
	jobCtx := <-executionContext
	cancel()
	if jobCtx.Err() != nil {
		t.Fatal("in-flight job was canceled before the drain deadline")
	}
	close(release)

	waitCtx, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	if err := pool.Wait(waitCtx); err != nil {
		t.Fatalf("workers did not stop: %v", err)
	}
	if processed.Load() != 2 {
		t.Fatalf("processed %d accepted jobs, want 2", processed.Load())
	}
	if !<-drainedWithLiveContext {
		t.Fatal("queued job received the already-cancelled worker context")
	}
	if err := pool.Submit(func(context.Context) error { return nil }); !errors.Is(err, workers.ErrPoolStopped) {
		t.Fatalf("submit after stop = %v, want ErrPoolStopped", err)
	}
}

func TestPanickingJobDeadLettersWithoutKillingWorker(t *testing.T) {
	counters := metrics.NewCounters(2)
	deadLettered := make(chan error, 1)
	var logs bytes.Buffer
	pool := workers.NewPool(workers.Options{
		Workers:     1,
		Capacity:    2,
		MaxAttempts: 1,
		Logger:      slog.New(slog.NewTextHandler(&logs, nil)),
		Metrics:     counters,
		DeadLetter: func(_ workers.Job, err error) {
			deadLettered <- err
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool.Start(ctx)

	if err := pool.Submit(func(context.Context) error { panic("boom") }); err != nil {
		t.Fatal(err)
	}
	var ran atomic.Bool
	if err := pool.Submit(func(context.Context) error { ran.Store(true); return nil }); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-deadLettered:
		if err == nil {
			t.Fatal("dead-letter error is nil")
		}
	case <-time.After(time.Second):
		t.Fatal("panicking job was not dead-lettered")
	}
	for deadline := time.Now().Add(time.Second); !ran.Load() && time.Now().Before(deadline); {
		time.Sleep(time.Millisecond)
	}
	if !ran.Load() {
		t.Fatal("worker did not process the job after a panic")
	}
	if got := counters.Snapshot(); got.JobsDeadLettered != 1 || got.JobsProcessed != 1 {
		t.Fatalf("unexpected counters: %+v", got)
	}
	if !strings.Contains(logs.String(), "job dead-lettered") {
		t.Fatalf("custom hook disabled dead-letter logging: %s", logs.String())
	}
}

func TestFailedJobRetriesUpToMaxAttempts(t *testing.T) {
	var attempts atomic.Int64
	deadLettered := make(chan struct{}, 1)
	pool := workers.NewPool(workers.Options{
		Workers:     1,
		Capacity:    1,
		MaxAttempts: 3,
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		DeadLetter: func(workers.Job, error) {
			deadLettered <- struct{}{}
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool.Start(ctx)
	if err := pool.Submit(func(context.Context) error {
		attempts.Add(1)
		return errors.New("retry")
	}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-deadLettered:
	case <-time.After(time.Second):
		t.Fatal("job was not dead-lettered")
	}
	if attempts.Load() != 3 {
		t.Fatalf("attempts = %d, want 3", attempts.Load())
	}
}

func TestDrainDeadlineDeadLettersRemainingQueuedJobs(t *testing.T) {
	started := make(chan struct{})
	var lastJobRan atomic.Bool
	deadLetters := make(chan struct{}, 2)
	pool := workers.NewPool(workers.Options{
		Workers:      1,
		Capacity:     3,
		DrainTimeout: 20 * time.Millisecond,
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		DeadLetter: func(workers.Job, error) {
			deadLetters <- struct{}{}
		},
	})
	if err := pool.Submit(func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := pool.Submit(func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}); err != nil {
		t.Fatal(err)
	}
	if err := pool.Submit(func(context.Context) error {
		lastJobRan.Store(true)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	pool.Start(ctx)
	<-started
	cancel()

	waitCtx, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	if err := pool.Wait(waitCtx); err != nil {
		t.Fatal(err)
	}
	if lastJobRan.Load() {
		t.Fatal("job ran after the drain deadline")
	}
	if len(deadLetters) != 2 {
		t.Fatalf("dead-lettered %d jobs, want 2", len(deadLetters))
	}
}

func TestDrainDeadlineAbandonsJobThatIgnoresCancellation_BR22(t *testing.T) {
	counters := metrics.NewCounters(4)
	started := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	pool := workers.NewPool(workers.Options{
		Workers:      1,
		Capacity:     4,
		DrainTimeout: 20 * time.Millisecond,
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		Metrics:      counters,
	})
	// Blocks on something unrelated to its context, as a syscall would.
	if err := pool.Submit(func(context.Context) error {
		close(started)
		<-release
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	pool.Start(ctx)
	<-started
	cancel()

	waitCtx, stop := context.WithTimeout(context.Background(), 2*time.Second)
	defer stop()
	if err := pool.Wait(waitCtx); !errors.Is(err, workers.ErrDrainIncomplete) {
		t.Fatalf("Wait = %v, want ErrDrainIncomplete", err)
	}
	if got := counters.Snapshot().JobsAbandoned; got != 1 {
		t.Fatalf("jobs abandoned = %d, want 1", got)
	}
}

func TestDrainDeadlineDeadLettersQueuedJobsDespiteStuckWorker_BR14(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	deadLetters := make(chan struct{}, 4)
	var queuedJobRan atomic.Bool
	pool := workers.NewPool(workers.Options{
		Workers:      1,
		Capacity:     4,
		DrainTimeout: 20 * time.Millisecond,
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		DeadLetter:   func(workers.Job, error) { deadLetters <- struct{}{} },
	})
	// Occupies the only worker, so nothing else is ever dequeued by one.
	if err := pool.Submit(func(context.Context) error {
		close(started)
		<-release
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := pool.Submit(func(context.Context) error {
			queuedJobRan.Store(true)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	pool.Start(ctx)
	<-started
	cancel()

	waitCtx, stop := context.WithTimeout(context.Background(), 2*time.Second)
	defer stop()
	if err := pool.Wait(waitCtx); !errors.Is(err, workers.ErrDrainIncomplete) {
		t.Fatalf("Wait = %v, want ErrDrainIncomplete", err)
	}
	if queuedJobRan.Load() {
		t.Fatal("queued job ran after the drain deadline")
	}
	if len(deadLetters) != 2 {
		t.Fatalf("dead-lettered %d queued jobs, want 2", len(deadLetters))
	}
}

func TestDrainDeadlineCompletesDespiteBlockingDeadLetterHook_BR14(t *testing.T) {
	counters := metrics.NewCounters(4)
	started := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	pool := workers.NewPool(workers.Options{
		Workers:      1,
		Capacity:     4,
		DrainTimeout: 20 * time.Millisecond,
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		Metrics:      counters,
		// A sink that stalls, as a network dead-letter target can.
		DeadLetter: func(workers.Job, error) { <-release },
	})
	if err := pool.Submit(func(context.Context) error {
		close(started)
		<-release
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := pool.Submit(func(context.Context) error { return nil }); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	pool.Start(ctx)
	<-started
	cancel()

	waitCtx, stop := context.WithTimeout(context.Background(), 2*time.Second)
	defer stop()
	if err := pool.Wait(waitCtx); !errors.Is(err, workers.ErrDrainIncomplete) {
		t.Fatalf("Wait = %v, want ErrDrainIncomplete", err)
	}
	if got := counters.Snapshot().JobsAbandoned; got != 3 {
		t.Fatalf("jobs abandoned = %d, want 3", got)
	}
}

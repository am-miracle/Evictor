package app

import (
	"context"
	"time"
)

// ShutdownBudget anchors one process-wide shutdown deadline to cancellation.
// CleanupMargin is included after the drain timeout for cancellation bookkeeping.
type ShutdownBudget struct {
	parent  context.Context
	timeout time.Duration
	margin  time.Duration
	now     func() time.Time
	ready   chan struct{}
	end     time.Time
}

func NewShutdownBudget(parent context.Context, timeout, cleanupMargin time.Duration) *ShutdownBudget {
	return newShutdownBudget(parent, timeout, cleanupMargin, time.Now)
}

func newShutdownBudget(
	parent context.Context,
	timeout time.Duration,
	cleanupMargin time.Duration,
	now func() time.Time,
) *ShutdownBudget {
	budget := &ShutdownBudget{
		parent:  parent,
		timeout: timeout,
		margin:  cleanupMargin,
		now:     now,
		ready:   make(chan struct{}),
	}
	go func() {
		<-parent.Done()
		budget.end = now().Add(timeout + cleanupMargin)
		close(budget.ready)
	}()
	return budget
}

// Context returns a deadline context for awaiting shutdown completion. If the
// process is not shutting down, the budget starts now rather than blocking.
func (b *ShutdownBudget) Context() (context.Context, context.CancelFunc) {
	return context.WithDeadline(context.Background(), b.deadline())
}

func (b *ShutdownBudget) deadline() time.Time {
	if b.parent.Err() == nil {
		return b.now().Add(b.timeout + b.margin)
	}
	<-b.ready
	return b.end
}

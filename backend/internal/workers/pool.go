package workers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"github.com/am-miracle/evictor/internal/metrics"
)

var (
	ErrQueueFull   = errors.New("worker queue full")
	ErrPoolStopped = errors.New("worker pool stopped")
)

type Job func(context.Context) error
type DeadLetterFunc func(Job, error)

type Options struct {
	Workers      int
	Capacity     int
	MaxAttempts  int
	DrainTimeout time.Duration
	Logger       *slog.Logger
	Metrics      *metrics.Counters
	DeadLetter   DeadLetterFunc
}

type Pool struct {
	jobs         chan Job
	workers      int
	maxAttempts  int
	drainTimeout time.Duration
	logger       *slog.Logger
	metrics      *metrics.Counters
	deadLetter   DeadLetterFunc
	startOnce    sync.Once
	done         chan struct{}
	stateMu      sync.Mutex
	stopping     bool
	executionCtx context.Context
	cancelJobs   context.CancelFunc
}

func NewPool(options Options) *Pool {
	if options.Workers <= 0 {
		options.Workers = 1
	}
	if options.Capacity <= 0 {
		options.Capacity = 1
	}
	if options.MaxAttempts <= 0 {
		options.MaxAttempts = 1
	}
	if options.DrainTimeout <= 0 {
		options.DrainTimeout = 15 * time.Second
	}
	if options.Logger == nil {
		options.Logger = slog.Default()
	}
	if options.Metrics == nil {
		options.Metrics = metrics.NewCounters(options.Capacity)
	}
	return &Pool{
		jobs:         make(chan Job, options.Capacity),
		workers:      options.Workers,
		maxAttempts:  options.MaxAttempts,
		drainTimeout: options.DrainTimeout,
		logger:       options.Logger,
		metrics:      options.Metrics,
		deadLetter:   options.DeadLetter,
		done:         make(chan struct{}),
	}
}

func (p *Pool) Submit(job Job) error {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()
	if p.stopping {
		return ErrPoolStopped
	}
	select {
	case p.jobs <- job:
		p.metrics.AddQueueDepth(1)
		return nil
	default:
		return ErrQueueFull
	}
}

func (p *Pool) Start(ctx context.Context) {
	p.startOnce.Do(func() {
		p.executionCtx, p.cancelJobs = context.WithCancel(context.WithoutCancel(ctx))
		var workers sync.WaitGroup
		workers.Add(p.workers)
		for range p.workers {
			go func() {
				defer workers.Done()
				p.work()
			}()
		}

		workersDone := make(chan struct{})
		go func() {
			workers.Wait()
			close(workersDone)
		}()

		go func() {
			<-ctx.Done()
			p.stopAccepting()

			timer := time.NewTimer(p.drainTimeout)
			defer timer.Stop()
			select {
			case <-workersDone:
			case <-timer.C:
				p.cancelJobs()
				<-workersDone
			}
			p.cancelJobs()
			close(p.done)
		}()
	})
}

func (p *Pool) Wait(ctx context.Context) error {
	select {
	case <-p.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *Pool) stopAccepting() {
	p.stateMu.Lock()
	if !p.stopping {
		p.stopping = true
		close(p.jobs)
	}
	p.stateMu.Unlock()
}

func (p *Pool) work() {
	for job := range p.jobs {
		p.jobDequeued()
		if err := p.executionCtx.Err(); err != nil {
			p.metrics.JobFailed()
			p.metrics.JobDeadLettered()
			p.recordDeadLetter(job, err)
			continue
		}
		p.execute(p.executionCtx, job)
	}
}

func (p *Pool) jobDequeued() {
	p.stateMu.Lock()
	p.metrics.AddQueueDepth(-1)
	p.stateMu.Unlock()
}

func (p *Pool) execute(ctx context.Context, job Job) {
	var err error
	for attempt := 1; attempt <= p.maxAttempts; attempt++ {
		err = runJob(ctx, job)
		if err == nil {
			p.metrics.JobProcessed()
			return
		}
		p.logger.Warn("job attempt failed", "attempt", attempt, "max_attempts", p.maxAttempts, "error", err)
		if ctx.Err() != nil {
			break
		}
	}
	p.metrics.JobFailed()
	p.metrics.JobDeadLettered()
	p.recordDeadLetter(job, err)
}

func (p *Pool) recordDeadLetter(job Job, err error) {
	p.logger.Error("job dead-lettered", "error", err)
	if p.deadLetter != nil {
		p.deadLetter(job, err)
	}
}

func runJob(ctx context.Context, job Job) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("job panic: %v\n%s", recovered, debug.Stack())
		}
	}()
	return job(ctx)
}

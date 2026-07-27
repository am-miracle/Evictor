package workers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/am-miracle/evictor/internal/metrics"
)

var (
	ErrQueueFull       = errors.New("worker queue full")
	ErrPoolStopped     = errors.New("worker pool stopped")
	ErrDrainIncomplete = errors.New("worker drain incomplete")
)

// Grace for cancelled jobs to return, so the abandoned count does not race
// jobs that are already unwinding.
const unwindWindow = 100 * time.Millisecond

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
	inFlight     atomic.Int64
	abandoned    atomic.Int64
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
				// Recording the backlog here rather than leaving it to the
				// workers keeps one stuck job from burying the rest (BR-14),
				// and the bounded wait keeps a stalled job or dead-letter sink
				// from holding the pool open.
				p.cancelJobs()
				settled := make(chan struct{})
				go func() {
					p.drainQueue()
					<-workersDone
					close(settled)
				}()
				unwind := time.NewTimer(unwindWindow)
				defer unwind.Stop()
				select {
				case <-settled:
				case <-unwind.C:
					p.recordAbandoned()
				}
			}
			p.cancelJobs()
			close(p.done)
		}()
	})
}

// Wait reports ErrDrainIncomplete when work was left unaccounted for at the
// drain deadline, so callers can surface the loss instead of assuming success.
func (p *Pool) Wait(ctx context.Context) error {
	select {
	case <-p.done:
		if abandoned := p.abandoned.Load(); abandoned > 0 {
			return fmt.Errorf("%w: %d job(s) unaccounted for at the drain deadline", ErrDrainIncomplete, abandoned)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Workers consume the same closed channel, so each job is recorded exactly
// once whichever gets to it first.
func (p *Pool) drainQueue() {
	for job := range p.jobs {
		p.jobDequeued()
		p.recordDeadLetter(job, p.executionCtx.Err())
	}
}

func (p *Pool) recordAbandoned() {
	count := p.inFlight.Load() + int64(len(p.jobs))
	if count <= 0 {
		return
	}
	p.abandoned.Store(count)
	p.metrics.AddJobsAbandoned(int(count))
	p.logger.Error("drain deadline expired with work unaccounted for", "abandoned", count)
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
	p.inFlight.Add(1)
	defer p.inFlight.Add(-1)
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
	p.recordDeadLetter(job, err)
}

func (p *Pool) recordDeadLetter(job Job, err error) {
	p.metrics.JobFailed()
	p.metrics.JobDeadLettered()
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

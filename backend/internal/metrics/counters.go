package metrics

import "sync/atomic"

type Snapshot struct {
	QueueDepth       int64 `json:"queue_depth"`
	QueueCapacity    int64 `json:"queue_capacity"`
	JobsProcessed    int64 `json:"jobs_processed"`
	JobsFailed       int64 `json:"jobs_failed"`
	JobsDeadLettered int64 `json:"jobs_dead_lettered"`
	JobsAbandoned    int64 `json:"jobs_abandoned"`
}

type Counters struct {
	queueDepth       atomic.Int64
	queueCapacity    int64
	jobsProcessed    atomic.Int64
	jobsFailed       atomic.Int64
	jobsDeadLettered atomic.Int64
	jobsAbandoned    atomic.Int64
}

func NewCounters(queueCapacity int) *Counters {
	return &Counters{queueCapacity: int64(queueCapacity)}
}

func (c *Counters) SetQueueDepth(depth int) { c.queueDepth.Store(int64(depth)) }
func (c *Counters) AddQueueDepth(delta int) { c.queueDepth.Add(int64(delta)) }
func (c *Counters) JobProcessed()           { c.jobsProcessed.Add(1) }
func (c *Counters) JobFailed()              { c.jobsFailed.Add(1) }
func (c *Counters) JobDeadLettered()        { c.jobsDeadLettered.Add(1) }

func (c *Counters) AddJobsAbandoned(count int) { c.jobsAbandoned.Add(int64(count)) }

func (c *Counters) Snapshot() Snapshot {
	return Snapshot{
		QueueDepth:       c.queueDepth.Load(),
		QueueCapacity:    c.queueCapacity,
		JobsProcessed:    c.jobsProcessed.Load(),
		JobsFailed:       c.jobsFailed.Load(),
		JobsDeadLettered: c.jobsDeadLettered.Load(),
		JobsAbandoned:    c.jobsAbandoned.Load(),
	}
}

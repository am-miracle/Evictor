package metrics_test

import (
	"testing"

	"github.com/am-miracle/evictor/internal/metrics"
)

func TestCountersReportQueueAndJobState(t *testing.T) {
	counters := metrics.NewCounters(12)
	counters.SetQueueDepth(3)
	counters.AddQueueDepth(2)
	counters.JobProcessed()
	counters.JobFailed()
	counters.JobDeadLettered()
	counters.AddJobsAbandoned(2)

	got := counters.Snapshot()
	if got.QueueDepth != 5 || got.QueueCapacity != 12 || got.JobsProcessed != 1 ||
		got.JobsFailed != 1 || got.JobsDeadLettered != 1 || got.JobsAbandoned != 2 {
		t.Fatalf("unexpected snapshot: %+v", got)
	}
}

package handlers

import (
	"fmt"
	"net/http"

	"github.com/am-miracle/evictor/internal/metrics"
)

func metricsHandler(counters *metrics.Counters) http.HandlerFunc {
	return func(response http.ResponseWriter, _ *http.Request) {
		snapshot := counters.Snapshot()
		response.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = fmt.Fprintf(response,
			"evictor_queue_depth %d\n"+
				"evictor_queue_capacity %d\n"+
				"evictor_jobs_processed_total %d\n"+
				"evictor_jobs_failed_total %d\n"+
				"evictor_jobs_dead_lettered_total %d\n"+
				"evictor_jobs_abandoned_total %d\n",
			snapshot.QueueDepth,
			snapshot.QueueCapacity,
			snapshot.JobsProcessed,
			snapshot.JobsFailed,
			snapshot.JobsDeadLettered,
			snapshot.JobsAbandoned,
		)
	}
}

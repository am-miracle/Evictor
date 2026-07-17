package workers

import "context"

// Run owns the background-worker process lifecycle. Work queues are added by
// later milestones; keeping the role separate now prevents API request load
// from competing with background processing.
func Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

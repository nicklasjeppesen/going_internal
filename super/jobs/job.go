//-----------------------------------------------------------------
// 							Jobs
//-----------------------------------------------------------------
//
// Job represents a unit of work executed periodically by the Scheduler.
// The execution context is stored on the Job instance so it can be read
// by the Runner without mutating shared state across goroutines.

package jobs

import (
	"context"
	"time"

	channels "github.com/nicklasjeppesen/going_internal/super/channels"
)

// Job describes a periodic task managed by the Scheduler.
type Job struct {

	// Title of the job
	Title string

	// Go's context
	ctx context.Context

	// Runner is the function executed for each scheduled run.
	Runner func(Job)

	// Interval controls how often the job is triggered.
	Interval time.Duration

	// TerminateAfter defines a per-execution timeout.
	// If zero, the job only observes the scheduler's cancellation context.
	TerminateAfter time.Duration
}

// Ctx returns the context associated with the job execution.
func (job *Job) Ctx() context.Context {
	return job.ctx
}

// NewJob creates a Job with the provided context.
func NewJob(_ctx context.Context) *Job {
	return &Job{ctx: _ctx}
}

// SendMessageToSocket sends a message to a websocket socket via the message provider.
func (job *Job) SendMessageToSocket(websocket channels.Socket) {
	provider := channels.WebSocketMessageProvider{}
	provider.SendMessageToSocket(websocket)
}

// IsInterrupted reports whether the job context has been cancelled
// or its deadline has been exceeded.
// It returns true together with the underlying context error if interrupted.
func (job *Job) IsInterrupted() (bool, error) {
	select {
	case <-job.ctx.Done():
		return true, job.ctx.Err()
	default:
		return false, nil
	}
}

// InterruptedByDeadlineExceeded reports whether the job stopped because
// its context deadline was exceeded.
func (job *Job) InterruptedByDeadlineExceeded() bool {
	_, err := job.IsInterrupted()
	return err == context.DeadlineExceeded
}

// InterruptedByCanceled reports whether the job stopped because
// its context was cancelled.
func (job *Job) InterruptedByCanceled() bool {
	_, err := job.IsInterrupted()
	return err == context.Canceled
}

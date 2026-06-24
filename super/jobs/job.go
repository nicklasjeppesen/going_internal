package jobs

import (
	"context"
	"time"

	channels "github.com/nicklasjeppesen/going_internal/super/channels"
)

//-----------------------------------------------------------------
// 							Jobs
//-----------------------------------------------------------------
//
// Job defines a unit of work to be executed periodically by the Scheduler.
// Context is passed explicitly to the Runner rather than stored on the struct,
// avoiding concurrent mutation and making the data flow clear.

type Job struct {

	// Title of the job
	Title string

	// Go's context
	ctx context.Context

	// Runner is the function executed on each tick.
	Runner func(Job)

	// Interval controls how often the job is triggered.
	Interval time.Duration

	// TerminateAfter sets a per-execution deadline. If zero, no deadline
	// is applied beyond the scheduler's own context cancellation.
	TerminateAfter time.Duration
}

func (job *Job) Ctx() context.Context {
	return job.ctx
}

func NewJob(_ctx context.Context) *Job {
	return &Job{ctx: _ctx}
}

// SendMessageToSocket
//
// Send a message to a websocket hub
func (job *Job) SendMessageToSocket(websocket channels.Socket) {
	provider := channels.WebSocketMessageProvider{}
	provider.SendMessageToSocket(websocket)
}

// IsInterrupted reports whether the given context has been cancelled or
// has exceeded its deadline. It also returns the reason, if any.
func (job *Job) IsInterrupted() (bool, error) {
	select {
	case <-job.ctx.Done():
		return true, job.ctx.Err()
	default:
		return false, nil
	}
}

func (job *Job) InterruptedByDeadlineExceeded() bool {
	_, err := job.IsInterrupted()
	return err == context.DeadlineExceeded
}

func (job *Job) InterruptedByCanceled() bool {
	_, err := job.IsInterrupted()
	return err == context.Canceled
}

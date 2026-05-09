package jobs

import (
	"context"
	"time"

	channels "github.com/nicklasjeppesen/going_internal/internal/super/channels"
)

//-----------------------------------------------------------------
// 							Jobs
//-----------------------------------------------------------------
//
// Jobs is responsible for provding a data structure,
// for the Application scheduler
//
// Jobs is defined by defining af new Jobs struct

type Job struct {

	//
	// Go's context
	ctx context.Context

	//
	// Title of the job
	Title string

	//
	// The job that shall be executed in a time period
	Runner func(Job)

	//
	// How how shall the time be executed
	Interval time.Duration

	//
	//  deadline The time can tim
	TerminateAfter time.Duration

	//
	// Reason for interrupted
	InterruptedReason error
}

// SendMessageToSocket
//
// Send a message to a websocket hub
func (job *Job) SendMessageToSocket(websocket channels.Socket) {
	provider := channels.WebSocketMessageProvider{}
	provider.SendMessageToSocket(websocket)
}

// Helper function to let the job know if it has been interrupted
// interrupted can happen by a timeout fail or The server is shutting down
func (job *Job) IsInterrupted() bool {
	select {
	case <-job.ctx.Done():
		job.InterruptedReason = job.ctx.Err() // Set interrupted reason
		return true
	default:
		return false
	}
}

func (job *Job) InterruptedByDeadlineExceeded() bool {
	return job.InterruptedReason == context.DeadlineExceeded
}

func (job *Job) InterruptedByCanceled() bool { return job.InterruptedReason == context.Canceled }

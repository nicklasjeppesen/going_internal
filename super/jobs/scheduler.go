package jobs

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

//
//-----------------------------------------------------------------------------
// 							Scheduler
//-----------------------------------------------------------------------------
//
// Scheduler is responsible for scheduling periodic background tasks.
// Each job runs on its own ticker goroutine and is protected against
// overlapping executions. Stopping the scheduler gracefully waits for
// all active jobs to finish before returning.

// Scheduler data structure, to run a job in the background
type Scheduler struct {

	// ctx is cancelled when Stop is called, signalling all goroutines to exit.
	ctx context.Context

	// cancel stops the scheduler and all managed jobs.
	cancel context.CancelFunc

	// scheduler tracks the ticker goroutines created by CreateJob.
	scheduler sync.WaitGroup

	// jobs tracks individual job executions that are currently in flight.
	jobs sync.WaitGroup
}

// Create a new instance of a Scheduler.
func New() *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		ctx:    ctx,
		cancel: cancel,
	}
}

// CreateJob registers and immediately starts scheduling the given Job.
// Returns a JobHandle and an error — the error is non-nil if the scheduler
// has already been stopped.
func (s *Scheduler) CreateJob(job Job) error {

	job.ctx = s.ctx
	s.scheduler.Add(1)
	go func() {
		defer s.scheduler.Done()

		ticker := time.NewTicker(job.Interval)
		defer ticker.Stop()

		var jobIsRunning atomic.Bool

		for {
			select {
			case <-ticker.C: // Got a new ticket for starting a new job
				if jobIsRunning.Load() {
					log.Printf("job %q: already running, skipping ticket\n", job.Title)
					continue
				}

				// Guard: do not start a new execution if shutdown was signalled
				// between the ticker firing and reaching this point. Without this
				// check, s.jobs.Add(1) could race with the s.jobs.Wait() call
				// inside Stop(), which is explicitly unsafe per the sync package.
				select {
				case <-s.ctx.Done():
					return
				default:
				}

				var timoutContext context.Context
				var cancel context.CancelFunc

				if job.TerminateAfter != 0 {
					timoutContext, cancel = context.WithTimeout(s.ctx, job.TerminateAfter)
				} else {
					timoutContext, cancel = context.WithCancel(s.ctx)
				}
				jobIsRunning.Store(true)
				s.jobs.Add(1)

				// Capture a local copy of job so each goroutine has its own
				// isolated state. Setting ctx on the copy is therefore safe —
				// no other goroutine touches this particular copy.
				go func(copyJob Job, jobctx context.Context) {
					defer s.jobs.Done()
					defer cancel()
					defer jobIsRunning.Store(false)

					copyJob.ctx = jobctx
					copyJob.Runner(copyJob)

				}(job, timoutContext)
			case <-s.ctx.Done():
				fmt.Println("When a stop signal is received")
				return

			}
		}
	}()

	return nil
}

// Stop cancels the scheduler context and blocks until all ticker goroutines
// and all in-flight job executions have finished.
func (s *Scheduler) Stop() {
	// Signal all goroutines to stop accepting new work.
	s.cancel()

	// Wait for all ticker goroutines to exit. Once this returns, no further
	// calls to s.jobs.Add can occur, making the subsequent Wait safe.
	s.scheduler.Wait()

	// Wait for any job executions that were already in flight.
	s.jobs.Wait()
}

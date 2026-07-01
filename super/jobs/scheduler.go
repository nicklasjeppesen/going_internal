// -----------------------------------------------------------------------------
// Scheduler
// -----------------------------------------------------------------------------
//
// Scheduler triggers periodic background jobs and ensures that:
// - each job runs on its own ticker loop,
// - executions do not overlap,
// - shutdown waits for both ticker loops and active job executions to finish.
//
// package jobs contains background job scheduling utilities.
package jobs

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// Scheduler manages the lifecycle of periodic jobs.
type Scheduler struct {

	// ctx is cancelled when Stop is called. All ticker loops and jobs
	// should observe this context and exit gracefully.
	ctx context.Context

	// cancel stops the scheduler and signals all managed goroutines to shut down.
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

// CreateJob registers a periodic job and starts its scheduling loop immediately.
//
// The job will be triggered on each ticker tick, but a new execution will not
// start while a previous one is still running.
//
// Returns an error if the scheduler has already been stopped.
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

// Stop cancels the scheduler and waits until:
// - all ticker loops have exited,
// - all in-flight jobs have finished.
func (s *Scheduler) Stop() {
	// Signal all goroutines to stop accepting new work.
	s.cancel()

	// Wait for all ticker goroutines to exit. Once this returns, no further
	// calls to s.jobs.Add can occur, making the subsequent Wait safe.
	s.scheduler.Wait()

	// Wait for any job executions that were already in flight.
	s.jobs.Wait()
}

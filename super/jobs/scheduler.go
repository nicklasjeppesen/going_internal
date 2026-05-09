package jobs

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

//
//-----------------------------------------------------------------------------
// 							Scheduler
//-----------------------------------------------------------------------------
//
// Scheduler, is responsible for scheduling periodic bagground task
//
// Scheduler is responsible for running a Job.
//

// Scheduler data structure, to run a job in the background
type Scheduler struct {

	//
	// GO's context
	ctx context.Context

	//
	// context to cancle jobs
	cancel context.CancelFunc

	//
	// Waiting group for
	scheduler sync.WaitGroup

	//
	// Waiting group, for register a job is running
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

func (s *Scheduler) CreateJob(job Job) {

	job.ctx = s.ctx
	s.scheduler.Add(1)
	go func() {
		defer s.scheduler.Done()
		ticker := time.NewTicker(job.Interval)
		defer ticker.Stop()
		jobIsRunning := false

		for {
			select {
			case <-ticker.C: // Got a new ticket for starting a new job
				if jobIsRunning {
					log.Println("Already running, continue")
					continue
				}
				s.jobs.Add(1)

				var timoutContext context.Context
				var cancel context.CancelFunc
				if job.TerminateAfter != 0 {
					timoutContext, cancel = context.WithTimeout(s.ctx, job.TerminateAfter)
				} else {
					timoutContext, cancel = context.WithCancel(s.ctx)
				}
				jobIsRunning = true
				go func() {
					defer s.jobs.Done()
					defer cancel()
					job.ctx = timoutContext
					job.Runner(job)
					jobIsRunning = false
				}()
			case <-s.ctx.Done():
				fmt.Println("When a stop signal is received")
				jobIsRunning = false
				return

			}
		}
	}()
}

// Waiting for all jobs to finish
func (s *Scheduler) Stop() {
	// stop new jobs
	s.cancel()

	// Waiting for jobs to finish
	s.scheduler.Wait()

	// venting for active jobs
	s.jobs.Wait()
}

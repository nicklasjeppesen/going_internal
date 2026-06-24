package jobs

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/nicklasjeppesen/going_internal/super/jobs"
)

//-----------------------------------------------------------------------------
// Helpers
//-----------------------------------------------------------------------------

// shortInterval is used as the default ticker interval in tests.
const shortInterval = 20 * time.Millisecond

// waitFor polls cond every 5 ms until it returns true or the timeout expires.
// Returns true if cond was satisfied within the deadline.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

//-----------------------------------------------------------------------------
// Scheduler lifecycle
//-----------------------------------------------------------------------------

func TestStop_NoPanic(t *testing.T) {
	s := New()
	s.Stop() // stopping an empty scheduler must not panic or deadlock
}

func TestStop_CalledTwice_NoPanic(t *testing.T) {
	s := New()

	// The second cancel() call on the underlying context is a no-op in Go,
	// so this should never panic.
	s.Stop()
	s.Stop()
}

func TestStop_WaitsForInFlightJob(t *testing.T) {
	s := New()

	jobStarted := make(chan struct{})
	jobMayFinish := make(chan struct{})

	err := s.CreateJob(Job{
		Title:    "slow-job",
		Interval: shortInterval,
		Runner: func(j Job) {
			close(jobStarted)
			<-jobMayFinish // block until the test releases us
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Wait until the job has actually started.
	select {
	case <-jobStarted:
	case <-time.After(time.Second):
		t.Fatal("job never started")
	}

	// Unblock the job and then stop — Stop() must not return before the job
	// finishes because it waits on s.jobs.
	close(jobMayFinish)
	s.Stop()
}

//-----------------------------------------------------------------------------
// Job execution
//-----------------------------------------------------------------------------

func TestJob_RunsAtLeastOnce(t *testing.T) {
	s := New()
	defer s.Stop()

	var count atomic.Int32

	err := s.CreateJob(Job{
		Title:    "counter",
		Interval: shortInterval,
		Runner:   func(Job) { count.Add(1) },
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !waitFor(t, time.Second, func() bool { return count.Load() >= 1 }) {
		t.Fatalf("job never ran, count = %d", count.Load())
	}
}

func TestJob_RunsMultipleTimes(t *testing.T) {
	s := New()
	defer s.Stop()

	var count atomic.Int32

	err := s.CreateJob(Job{
		Title:    "multi-counter",
		Interval: shortInterval,
		Runner:   func(Job) { count.Add(1) },
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !waitFor(t, time.Second, func() bool { return count.Load() >= 3 }) {
		t.Fatalf("job did not run 3 times, count = %d", count.Load())
	}
}

func TestJob_NoOverlappingExecutions(t *testing.T) {
	s := New()
	defer s.Stop()

	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32

	err := s.CreateJob(Job{
		Title:    "overlap-check",
		Interval: shortInterval,
		Runner: func(j Job) {
			// Track how many invocations are running at the same time.
			current := concurrent.Add(1)
			for {
				old := maxConcurrent.Load()
				if current <= old || maxConcurrent.CompareAndSwap(old, current) {
					break
				}
			}
			// Simulate work that outlasts the interval so the scheduler would
			// try to launch a second invocation if overlap protection is broken.
			time.Sleep(shortInterval * 3)
			concurrent.Add(-1)
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Let several ticks fire.
	time.Sleep(shortInterval * 10)

	if max := maxConcurrent.Load(); max > 1 {
		t.Errorf("overlapping executions detected: %d ran concurrently, want ≤ 1", max)
	}
}

func TestJob_StopsRunningAfterSchedulerStop(t *testing.T) {
	s := New()

	var count atomic.Int32

	err := s.CreateJob(Job{
		Title:    "stop-check",
		Interval: shortInterval,
		Runner:   func(Job) { count.Add(1) },
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Let it run a few times, then stop.
	if !waitFor(t, time.Second, func() bool { return count.Load() >= 2 }) {
		t.Fatalf("job never ran enough times before stop, count = %d", count.Load())
	}

	s.Stop()
	snapshot := count.Load()

	// After Stop() the count must not increase.
	time.Sleep(shortInterval * 5)
	if after := count.Load(); after > snapshot+1 {
		// Allow at most one extra execution that was already in flight at Stop time.
		t.Errorf("job kept running after Stop: count went from %d to %d", snapshot, after)
	}
}

//-----------------------------------------------------------------------------
// TerminateAfter deadline
//-----------------------------------------------------------------------------

func TestJob_TerminateAfter_CancelsSlowJob(t *testing.T) {
	s := New()
	defer s.Stop()

	interrupted := make(chan error, 1)

	err := s.CreateJob(Job{
		Title:          "deadline-job",
		Interval:       shortInterval,
		TerminateAfter: 30 * time.Millisecond,
		Runner: func(j Job) {
			// Block until our context is cancelled by TerminateAfter.
			select {
			case <-time.After(10 * time.Second): // would time out the test
			case <-j.Ctx().Done(): // context cancelled by scheduler
				_, reason := j.IsInterrupted()
				interrupted <- reason
			}
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case reason := <-interrupted:
		if reason != context.DeadlineExceeded {
			t.Errorf("expected DeadlineExceeded, got %v", reason)
		}
	case <-time.After(time.Second):
		t.Fatal("job was not interrupted by TerminateAfter within 1 s")
	}
}

//-----------------------------------------------------------------------------
// Context helpers on Job
//-----------------------------------------------------------------------------

func TestIsInterrupted_FreshContext_ReturnsFalse(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	j := NewJob(ctx)
	interrupted, err := j.IsInterrupted()

	if interrupted {
		t.Errorf("expected not interrupted, got true")
	}
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestIsInterrupted_CancelledContext_ReturnsTrue(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	j := NewJob(ctx)
	interrupted, err := j.IsInterrupted()

	if !interrupted {
		t.Error("expected interrupted = true")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestIsInterrupted_DeadlineExceeded_ReturnsTrue(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	time.Sleep(10 * time.Millisecond) // let the deadline pass

	j := NewJob(ctx)
	interrupted, err := j.IsInterrupted()

	if !interrupted {
		t.Error("expected interrupted = true")
	}
	if err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
}

func TestInterruptedByDeadlineExceeded(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	time.Sleep(10 * time.Millisecond)

	j := NewJob(ctx)
	if !j.InterruptedByDeadlineExceeded() {
		t.Error("expected InterruptedByDeadlineExceeded = true")
	}
	if j.InterruptedByCanceled() {
		t.Error("expected InterruptedByCanceled = false")
	}
}

func TestInterruptedByCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	j := NewJob(ctx)
	if !j.InterruptedByCanceled() {
		t.Error("expected InterruptedByCanceled = true")
	}
	if j.InterruptedByDeadlineExceeded() {
		t.Error("expected InterruptedByDeadlineExceeded = false")
	}
}

//-----------------------------------------------------------------------------
// Multiple jobs
//-----------------------------------------------------------------------------

func TestMultipleJobs_AllRun(t *testing.T) {
	s := New()
	defer s.Stop()

	const numJobs = 5
	counts := make([]atomic.Int32, numJobs)

	for i := range numJobs {
		idx := i
		err := s.CreateJob(Job{
			Title:    "job",
			Interval: shortInterval,
			Runner:   func(Job) { counts[idx].Add(1) },
		})
		if err != nil {
			t.Fatalf("job %d: unexpected error: %v", idx, err)
		}
	}

	for i := range numJobs {
		idx := i
		if !waitFor(t, time.Second, func() bool { return counts[idx].Load() >= 1 }) {
			t.Errorf("job %d never ran", idx)
		}
	}
}

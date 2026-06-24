package jobs

import (
	"context"
	"testing"
	"time"

	. "github.com/nicklasjeppesen/going_internal/super/jobs"
)

//-----------------------------------------------------------------------------
// IsInterrupted
//-----------------------------------------------------------------------------

func TestIsInterrupted_ActiveContext_ReturnsFalse(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	j := NewJob(ctx)
	interrupted, err := j.IsInterrupted()

	if interrupted {
		t.Error("expected interrupted = false for an active context")
	}
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestIsInterrupted_CancelledContext_ReturnsTrueWithCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	j := NewJob(ctx)
	interrupted, err := j.IsInterrupted()

	if !interrupted {
		t.Error("expected interrupted = true after cancel")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestIsInterrupted_ExpiredDeadline_ReturnsTrueWithDeadlineExceeded(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	time.Sleep(10 * time.Millisecond) // let the deadline pass

	j := NewJob(ctx)
	interrupted, err := j.IsInterrupted()

	if !interrupted {
		t.Error("expected interrupted = true after deadline exceeded")
	}
	if err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
}

// Calling IsInterrupted multiple times on the same job must always return the
// same result — it must never mutate shared state.
func TestIsInterrupted_Idempotent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	j := NewJob(ctx)

	for range 5 {
		interrupted, err := j.IsInterrupted()
		if !interrupted {
			t.Error("expected interrupted = true on repeated call")
		}
		if err != context.Canceled {
			t.Errorf("expected context.Canceled on repeated call, got %v", err)
		}
	}
}

//-----------------------------------------------------------------------------
// InterruptedByDeadlineExceeded
//-----------------------------------------------------------------------------

func TestInterruptedByDeadlineExceeded_ActiveContext_ReturnsFalse(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	j := NewJob(ctx)
	if j.InterruptedByDeadlineExceeded() {
		t.Error("expected false for an active context")
	}
}

func TestInterruptedByDeadlineExceeded_CancelledContext_ReturnsFalse(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	j := NewJob(ctx)
	if j.InterruptedByDeadlineExceeded() {
		t.Error("expected false when interrupted by cancel, not deadline")
	}
}

func TestInterruptedByDeadlineExceeded_ExpiredDeadline_ReturnsTrue(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	time.Sleep(10 * time.Millisecond)

	j := NewJob(ctx)
	if !j.InterruptedByDeadlineExceeded() {
		t.Error("expected true after deadline exceeded")
	}
}

//-----------------------------------------------------------------------------
// InterruptedByCanceled
//-----------------------------------------------------------------------------

func TestInterruptedByCanceled_ActiveContext_ReturnsFalse(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	j := NewJob(ctx)
	if j.InterruptedByCanceled() {
		t.Error("expected false for an active context")
	}
}

func TestInterruptedByCanceled_CancelledContext_ReturnsTrue(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	j := NewJob(ctx)
	if !j.InterruptedByCanceled() {
		t.Error("expected true after cancel")
	}
}

func TestInterruptedByCanceled_ExpiredDeadline_ReturnsFalse(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	time.Sleep(10 * time.Millisecond)

	j := NewJob(ctx)
	if j.InterruptedByCanceled() {
		t.Error("expected false when interrupted by deadline, not cancel")
	}
}

//-----------------------------------------------------------------------------
// Mutual exclusion between the two helpers
//-----------------------------------------------------------------------------

// Only one of the two helpers can return true at a time.
func TestInterruptedHelpers_MutuallyExclusive_OnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	j := NewJob(ctx)
	if j.InterruptedByDeadlineExceeded() && j.InterruptedByCanceled() {
		t.Error("both helpers returned true simultaneously — they must be mutually exclusive")
	}
}

func TestInterruptedHelpers_MutuallyExclusive_OnDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	time.Sleep(10 * time.Millisecond)

	j := NewJob(ctx)
	if j.InterruptedByDeadlineExceeded() && j.InterruptedByCanceled() {
		t.Error("both helpers returned true simultaneously — they must be mutually exclusive")
	}
}

//-----------------------------------------------------------------------------
// Ctx getter
//-----------------------------------------------------------------------------

func TestCtx_ReturnsUnderlyingContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	j := NewJob(ctx)
	if j.Ctx() != ctx {
		t.Error("Ctx() did not return the context that was set on the job")
	}
}

func TestCtx_CancelPropagates(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	j := NewJob(ctx)
	cancel()

	select {
	case <-j.Ctx().Done():
		// expected
	default:
		t.Error("cancelling the original context did not propagate through Ctx()")
	}
}

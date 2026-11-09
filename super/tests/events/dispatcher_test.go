package events_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nicklasjeppesen/going_internal/super/events"
)

// userRegistered is a small stand in for a real domain event.
type userRegistered struct {
	Email string
}

func (userRegistered) EventName() string {
	return "user.registered"
}

// userDeleted is a second event name, used to prove the routing keeps them
// apart.
type userDeleted struct {
	ID int64
}

func (userDeleted) EventName() string {
	return "user.deleted"
}

func TestDispatchCallsTheListenerOfTheEvent(t *testing.T) {
	dispatcher := events.New()
	defer dispatcher.Shutdown()

	var got string
	events.ListenTo(dispatcher, func(_ context.Context, event userRegistered) error {
		got = event.Email
		return nil
	})

	if err := dispatcher.Dispatch(context.Background(), userRegistered{Email: "a@b.dk"}); err != nil {
		t.Fatalf("Dispatch returned %v, want nil", err)
	}

	if want := "a@b.dk"; got != want {
		t.Errorf("listener received %q, want %q", got, want)
	}
}

func TestDispatchOnlyCallsTheListenerOfItsOwnEvent(t *testing.T) {
	dispatcher := events.New()
	defer dispatcher.Shutdown()

	var called bool
	events.ListenTo(dispatcher, func(_ context.Context, _ userRegistered) error {
		called = true
		return nil
	})

	if err := dispatcher.Dispatch(context.Background(), userDeleted{ID: 7}); err != nil {
		t.Fatalf("Dispatch returned %v, want nil", err)
	}

	if called {
		t.Error("the listener of user.registered was called for user.deleted")
	}

	if dispatcher.HasListeners(userRegistered{}.EventName()) != true {
		t.Error("HasListeners(user.registered) = false, want true")
	}
}

func TestDispatchCallsEveryListenerInOrder(t *testing.T) {
	dispatcher := events.New()
	defer dispatcher.Shutdown()

	var order []string
	for _, name := range []string{"first", "second", "third"} {
		dispatcher.Listen(userRegistered{}.EventName(), func(context.Context, events.Event) error {
			order = append(order, name)
			return nil
		})
	}

	dispatcher.Dispatch(context.Background(), userRegistered{})

	if len(order) != 3 || order[0] != "first" || order[1] != "second" || order[2] != "third" {
		t.Errorf("listeners ran as %v, want [first second third]", order)
	}
}

func TestDispatchRunsEveryListenerEvenWhenOneFails(t *testing.T) {
	dispatcher := events.New()
	defer dispatcher.Shutdown()

	var boom = errors.New("boom")
	var secondCalled bool

	dispatcher.Listen(userRegistered{}.EventName(), func(context.Context, events.Event) error {
		return boom
	})
	dispatcher.Listen(userRegistered{}.EventName(), func(context.Context, events.Event) error {
		secondCalled = true
		return nil
	})

	err := dispatcher.Dispatch(context.Background(), userRegistered{})

	if !errors.Is(err, boom) {
		t.Errorf("Dispatch returned %v, want it to wrap %v", err, boom)
	}

	if !secondCalled {
		t.Error("the failing listener stopped the next one")
	}
}

func TestDispatchRecoversAPanickingListener(t *testing.T) {
	dispatcher := events.New()
	defer dispatcher.Shutdown()

	dispatcher.Listen(userRegistered{}.EventName(), func(context.Context, events.Event) error {
		panic("listener is broken")
	})

	err := dispatcher.Dispatch(context.Background(), userRegistered{})

	if err == nil {
		t.Fatal("Dispatch returned nil, want the panic reported as an error")
	}
}

func TestDispatchSkipsTheListenersOnACancelledContext(t *testing.T) {
	dispatcher := events.New()
	defer dispatcher.Shutdown()

	var called bool
	dispatcher.Listen(userRegistered{}.EventName(), func(context.Context, events.Event) error {
		called = true
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := dispatcher.Dispatch(ctx, userRegistered{})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Dispatch returned %v, want it to wrap context.Canceled", err)
	}

	if called {
		t.Error("a listener was called on a cancelled context")
	}
}

func TestListenAllIsCalledForEveryEvent(t *testing.T) {
	dispatcher := events.New()
	defer dispatcher.Shutdown()

	var names []string
	dispatcher.ListenAll(func(_ context.Context, event events.Event) error {
		names = append(names, event.EventName())
		return nil
	})

	dispatcher.Dispatch(context.Background(), userRegistered{})
	dispatcher.Dispatch(context.Background(), userDeleted{})

	if len(names) != 2 || names[0] != "user.registered" || names[1] != "user.deleted" {
		t.Errorf("the wildcard listener saw %v, want both event names", names)
	}
}

func TestListenToDerivesTheNameFromTheEventType(t *testing.T) {
	dispatcher := events.New()
	defer dispatcher.Shutdown()

	var called bool
	events.ListenTo(dispatcher, func(context.Context, userDeleted) error {
		called = true
		return nil
	})

	if !dispatcher.HasListeners("user.deleted") {
		t.Error(`HasListeners("user.deleted") = false, want true`)
	}

	if dispatcher.HasListeners("user.registered") {
		t.Error(`HasListeners("user.registered") = true, want false`)
	}

	if err := dispatcher.Dispatch(context.Background(), userDeleted{ID: 3}); err != nil {
		t.Fatalf("Dispatch returned %v, want nil", err)
	}

	if !called {
		t.Error("the listener was not called for its own event")
	}
}

func TestPublishHandsTheEventToAWorker(t *testing.T) {
	dispatcher := events.New()
	defer dispatcher.Shutdown()

	var handled = make(chan string, 1)
	events.ListenTo(dispatcher, func(_ context.Context, event userRegistered) error {
		handled <- event.Email
		return nil
	})

	if err := dispatcher.Publish(context.Background(), userRegistered{Email: "a@b.dk"}); err != nil {
		t.Fatalf("Publish returned %v, want nil", err)
	}

	select {
	case email := <-handled:
		if want := "a@b.dk"; email != want {
			t.Errorf("listener received %q, want %q", email, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the published event")
	}
}

func TestPublishReportsFailuresOnTheDispatcher(t *testing.T) {
	var boom = errors.New("boom")

	// The error handler runs on a worker, so it shares state with the test
	// through the mutex.
	var mu sync.Mutex
	var reported []error

	dispatcher := events.New(events.WithErrorHandler(func(err error) {
		mu.Lock()
		reported = append(reported, err)
		mu.Unlock()
	}))
	defer dispatcher.Shutdown()

	dispatcher.Listen(userRegistered{}.EventName(), func(context.Context, events.Event) error {
		return boom
	})

	dispatcher.Publish(context.Background(), userRegistered{})

	// The worker may still be running, so wait for the error to show up.
	first := waitForFailure(t, dispatcher, 1)

	if !errors.Is(first, boom) {
		t.Errorf("the error handler received %v, want it to wrap %v", first, boom)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(reported) != 1 || !errors.Is(reported[0], boom) {
		t.Errorf("the error handler was called with %v, want one entry wrapping %v", reported, boom)
	}

	if dropped := dispatcher.ClearFailures(); dropped != 1 {
		t.Errorf("ClearFailures dropped %d entries, want 1", dropped)
	}

	if len(dispatcher.Failures()) != 0 {
		t.Error("Failures is not empty after ClearFailures")
	}
}

// waitForFailure waits until the dispatcher collected the wanted number of
// failures and returns the first one.
func waitForFailure(t *testing.T, dispatcher *events.Dispatcher, want int) error {
	t.Helper()

	deadline := time.After(2 * time.Second)
	for {
		if failures := dispatcher.Failures(); len(failures) >= want {
			return failures[0]
		}

		select {
		case <-deadline:
			t.Fatalf("timed out waiting for %d failures", want)
			return nil
		default:
			time.Sleep(time.Millisecond)
		}
	}
}

func TestPublishDetachedSurvivesACancelledCaller(t *testing.T) {
	dispatcher := events.New()
	defer dispatcher.Shutdown()

	var handled = make(chan string, 1)
	events.ListenTo(dispatcher, func(_ context.Context, event userRegistered) error {
		handled <- event.Email
		return nil
	})

	// The caller is gone, which is what happens to a http request the moment
	// its response has been sent.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := dispatcher.PublishDetached(ctx, userRegistered{Email: "a@b.dk"}); err != nil {
		t.Fatalf("PublishDetached returned %v, want nil", err)
	}

	select {
	case email := <-handled:
		if want := "a@b.dk"; email != want {
			t.Errorf("the listener received %q, want %q", email, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the listener was never called, PublishDetached kept the cancelled context")
	}
}

func TestPublishOnACancelledContextNeverCallsTheListener(t *testing.T) {
	dispatcher := events.New()
	defer dispatcher.Shutdown()

	var called atomic.Bool
	events.ListenTo(dispatcher, func(context.Context, userRegistered) error {
		called.Store(true)
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Publish may either refuse the event or queue it, but a cancelled context
	// must never reach a listener.
	dispatcher.Publish(ctx, userRegistered{Email: "a@b.dk"})

	// Give a worker the chance to pick the event up.
	time.Sleep(200 * time.Millisecond)

	if called.Load() {
		t.Error("the listener was called with a cancelled context")
	}
}

func TestPublishAfterShutdownIsRejected(t *testing.T) {
	dispatcher := events.New()
	dispatcher.Shutdown()

	err := dispatcher.Publish(context.Background(), userRegistered{})

	if !errors.Is(err, events.ErrShutdown) {
		t.Errorf("Publish returned %v, want events.ErrShutdown", err)
	}
}

func TestShutdownDrainsTheQueuedEvents(t *testing.T) {
	dispatcher := events.New(events.WithWorkers(1), events.WithBuffer(16))

	var mu sync.Mutex
	var handled int

	// The listener blocks, so the events after it pile up in the queue.
	dispatcher.Listen(userRegistered{}.EventName(), func(context.Context, events.Event) error {
		mu.Lock()
		handled++
		mu.Unlock()
		return nil
	})

	for i := 0; i < 5; i++ {
		if err := dispatcher.Publish(context.Background(), userRegistered{}); err != nil {
			t.Fatalf("Publish returned %v, want nil", err)
		}
	}

	dispatcher.Shutdown()

	mu.Lock()
	defer mu.Unlock()

	if want := 5; handled != want {
		t.Errorf("Shutdown handled %d events, want all %d", handled, want)
	}
}

func TestShutdownCanBeCalledMoreThanOnce(t *testing.T) {
	dispatcher := events.New()

	dispatcher.Shutdown()
	dispatcher.Shutdown()
}

func TestForgetDropsTheListener(t *testing.T) {
	dispatcher := events.New()
	defer dispatcher.Shutdown()

	var called bool
	dispatcher.Listen(userRegistered{}.EventName(), func(context.Context, events.Event) error {
		called = true
		return nil
	})

	dispatcher.Forget(userRegistered{}.EventName())
	dispatcher.Dispatch(context.Background(), userRegistered{})

	if called {
		t.Error("a forgotten listener was called")
	}

	if dispatcher.HasListeners(userRegistered{}.EventName()) {
		t.Error("HasListeners is true after Forget")
	}
}

func TestNamesReturnsTheRegisteredEvents(t *testing.T) {
	dispatcher := events.New()
	defer dispatcher.Shutdown()

	dispatcher.Listen("user.deleted", func(context.Context, events.Event) error { return nil })
	dispatcher.Listen("user.registered", func(context.Context, events.Event) error { return nil })

	names := dispatcher.Names()

	if len(names) != 2 || names[0] != "user.deleted" || names[1] != "user.registered" {
		t.Errorf("Names returned %v, want them sorted", names)
	}
}

func TestPublishFromManyGoroutines(t *testing.T) {
	dispatcher := events.New(events.WithWorkers(4), events.WithBuffer(64))
	defer dispatcher.Shutdown()

	var mu sync.Mutex
	var handled int

	dispatcher.Listen(userRegistered{}.EventName(), func(context.Context, events.Event) error {
		mu.Lock()
		handled++
		mu.Unlock()
		return nil
	})

	var publishers sync.WaitGroup
	for i := 0; i < 50; i++ {
		publishers.Add(1)
		go func() {
			defer publishers.Done()
			dispatcher.Publish(context.Background(), userRegistered{})
		}()
	}
	publishers.Wait()
	dispatcher.Shutdown()

	mu.Lock()
	defer mu.Unlock()

	if want := 50; handled != want {
		t.Errorf("handled %d events, want %d", handled, want)
	}
}

package events_test

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nicklasjeppesen/going_internal/super/events"
)

// Tests for named listeners: Name, Listeners and Deliver.

// mustPanic fails the test unless call panics with a message containing want.
func mustPanic(t *testing.T, want string, call func()) {
	t.Helper()
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatalf("expected a panic containing %q", want)
		}
		if message := recovered.(error).Error(); !strings.Contains(message, want) {
			t.Fatalf("panic %q, want it to contain %q", message, want)
		}
	}()
	call()
}

func TestListenersListsTheNamedListenersInOrder(t *testing.T) {
	dispatcher := events.New()
	defer dispatcher.Shutdown()

	noop := func(context.Context, userRegistered) error { return nil }
	events.On(dispatcher, noop, events.Name("log"))
	events.On(dispatcher, noop) // unnamed: left out
	events.On(dispatcher, noop, events.Async(), events.Name("mail"))
	events.ListenTo(dispatcher, noop, events.Name("follow")) // follows the caller: not Async
	events.On(dispatcher, func(context.Context, userDeleted) error { return nil }, events.Name("other"))
	dispatcher.ListenAll(func(context.Context, events.Event) error { return nil }) // unnamed wildcard: left out

	got := dispatcher.Listeners(userRegistered{})
	want := []events.ListenerInfo{{Name: "log"}, {Name: "mail", Async: true}, {Name: "follow"}}
	if len(got) != len(want) {
		t.Fatalf("Listeners: got %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Listeners[%d]: got %+v, want %+v", i, got[i], want[i])
		}
	}

	if none := dispatcher.Listeners(userRenamedEvent{}); len(none) != 0 {
		t.Fatalf("an event without named listeners: got %+v", none)
	}
}

// userRenamedEvent has no listeners at all.
type userRenamedEvent struct{}

func (userRenamedEvent) EventName() string { return "user.renamed" }

func TestDeliverRunsOnlyTheNamedListenerInline(t *testing.T) {
	dispatcher := events.New()
	defer dispatcher.Shutdown()

	var log, mail, unnamed atomic.Int32
	events.On(dispatcher, func(context.Context, userRegistered) error { log.Add(1); return nil }, events.Name("log"))
	events.On(dispatcher, func(context.Context, userRegistered) error { unnamed.Add(1); return nil })
	events.On(dispatcher, func(_ context.Context, event userRegistered) error {
		if event.Email != "a@b.dk" {
			t.Errorf("Deliver passed %+v", event)
		}
		mail.Add(1)
		return nil
	}, events.Async(), events.Name("mail"))

	// Async listeners run inline too, so the count is set when Deliver returns.
	if err := dispatcher.Deliver(context.Background(), userRegistered{Email: "a@b.dk"}, "mail"); err != nil {
		t.Fatal(err)
	}
	if mail.Load() != 1 || log.Load() != 0 || unnamed.Load() != 0 {
		t.Fatalf("mail %d, log %d, unnamed %d: want only mail", mail.Load(), log.Load(), unnamed.Load())
	}
}

func TestDeliverReturnsTheListenersErrorAndPanics(t *testing.T) {
	dispatcher := events.New()
	defer dispatcher.Shutdown()

	var errMail = errors.New("smtp down")
	events.On(dispatcher, func(context.Context, userRegistered) error { return errMail }, events.Name("mail"))
	events.On(dispatcher, func(context.Context, userRegistered) error { panic("boom") }, events.Name("broken"))

	if err := dispatcher.Deliver(context.Background(), userRegistered{}, "mail"); !errors.Is(err, errMail) {
		t.Fatalf("Deliver: got %v, want it to wrap %v", err, errMail)
	}
	if err := dispatcher.Deliver(context.Background(), userRegistered{}, "broken"); err == nil || !strings.Contains(err.Error(), "panicked: boom") {
		t.Fatalf("Deliver of a panicking listener: got %v", err)
	}
}

func TestDeliverWithoutSuchAListener(t *testing.T) {
	dispatcher := events.New()
	defer dispatcher.Shutdown()

	events.On(dispatcher, func(context.Context, userRegistered) error { return nil }, events.Name("log"))
	events.On(dispatcher, func(context.Context, userRegistered) error { return nil }) // unnamed

	for _, name := range []string{"mail", ""} {
		if err := dispatcher.Deliver(context.Background(), userRegistered{}, name); !errors.Is(err, events.ErrNoListener) {
			t.Fatalf("Deliver %q: got %v, want ErrNoListener", name, err)
		}
	}
	// "log" belongs to user.registered, not to user.deleted.
	if err := dispatcher.Deliver(context.Background(), userDeleted{}, "log"); !errors.Is(err, events.ErrNoListener) {
		t.Fatalf("Deliver to another event: got %v, want ErrNoListener", err)
	}
}

func TestDeliverReachesANamedWildcardListener(t *testing.T) {
	dispatcher := events.New()
	defer dispatcher.Shutdown()

	var audited []string
	dispatcher.ListenAll(func(_ context.Context, event events.Event) error {
		audited = append(audited, event.EventName())
		return nil
	}, events.Name("audit"))
	events.On(dispatcher, func(context.Context, userDeleted) error { return nil }, events.Name("log"))

	// The event's own listeners first, then the named wildcard one.
	if infos := dispatcher.Listeners(userDeleted{}); len(infos) != 2 || infos[0].Name != "log" || infos[1].Name != "audit" {
		t.Fatalf("Listeners with a named wildcard: %+v", infos)
	}
	if infos := dispatcher.Listeners(userRegistered{}); len(infos) != 1 || infos[0].Name != "audit" {
		t.Fatalf("a named wildcard listener belongs to every event: %+v", infos)
	}
	if err := dispatcher.Deliver(context.Background(), userDeleted{ID: 1}, "audit"); err != nil {
		t.Fatal(err)
	}
	if len(audited) != 1 || audited[0] != "user.deleted" {
		t.Fatalf("audited %v", audited)
	}
}

func TestDeliverAfterShutdown(t *testing.T) {
	dispatcher := events.New()
	events.On(dispatcher, func(context.Context, userRegistered) error { return nil }, events.Name("log"))
	dispatcher.Shutdown()

	if err := dispatcher.Deliver(context.Background(), userRegistered{}, "log"); !errors.Is(err, events.ErrShutdown) {
		t.Fatalf("Deliver after Shutdown: got %v, want ErrShutdown", err)
	}
}

func TestDeliverOnACancelledContextNeverCallsTheListener(t *testing.T) {
	dispatcher := events.New()
	defer dispatcher.Shutdown()

	var called bool
	events.On(dispatcher, func(context.Context, userRegistered) error { called = true; return nil }, events.Name("log"))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := dispatcher.Deliver(ctx, userRegistered{}, "log"); !errors.Is(err, context.Canceled) || called {
		t.Fatalf("Deliver on a cancelled context: err %v, called %v", err, called)
	}
}

func TestNameMustBeUniqueAndNotEmpty(t *testing.T) {
	dispatcher := events.New()
	defer dispatcher.Shutdown()

	noop := func(context.Context, userRegistered) error { return nil }
	events.On(dispatcher, noop, events.Name("log"))

	mustPanic(t, `"log" is already used`, func() { events.On(dispatcher, noop, events.Name("log")) })
	mustPanic(t, `"log" is already used`, func() { events.ListenTo(dispatcher, noop, events.Name("log")) })
	mustPanic(t, "empty name", func() { events.On(dispatcher, noop, events.Name("")) })

	// The same name on another event is fine.
	events.On(dispatcher, func(context.Context, userDeleted) error { return nil }, events.Name("log"))

	// A named wildcard listener is part of every event, so it can't reuse a
	// name any event has, and no event can reuse its name afterwards.
	mustPanic(t, `"log" is already used`, func() { dispatcher.ListenAll(nil, events.Name("log")) })
	dispatcher.ListenAll(func(context.Context, events.Event) error { return nil }, events.Name("audit"))
	mustPanic(t, `"audit" is already used`, func() { events.On(dispatcher, noop, events.Name("audit")) })

	if got := len(dispatcher.Listeners(userRegistered{})); got != 2 {
		t.Fatalf("a rejected registration must not be stored: %d named listeners", got)
	}
}

func TestPublishAndDispatchStillRunNamedAndUnnamedListeners(t *testing.T) {
	dispatcher := events.New()

	var sync, async, unnamed atomic.Int32
	events.On(dispatcher, func(context.Context, userRegistered) error { sync.Add(1); return nil }, events.Name("log"))
	events.On(dispatcher, func(context.Context, userRegistered) error { async.Add(1); return nil }, events.Async(), events.Name("mail"))
	events.On(dispatcher, func(context.Context, userRegistered) error { unnamed.Add(1); return nil })

	if err := dispatcher.Publish(context.Background(), userRegistered{}); err != nil {
		t.Fatal(err)
	}
	if err := dispatcher.PublishDetached(context.Background(), userRegistered{}); err != nil {
		t.Fatal(err)
	}
	if err := dispatcher.Dispatch(context.Background(), userRegistered{}); err != nil {
		t.Fatal(err)
	}
	dispatcher.Shutdown() // drains the async listener

	if sync.Load() != 3 || async.Load() != 3 || unnamed.Load() != 3 {
		t.Fatalf("sync %d, async %d, unnamed %d: want 3 each", sync.Load(), async.Load(), unnamed.Load())
	}
}

package events_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/nicklasjeppesen/going_internal/super/events"
)

// Tests for the names On and ListenTo derive from the listener's function
// when no Name is given.

// logRegistered is a listener declared as a named function.
func logRegistered(context.Context, userRegistered) error { return nil }

// mailRegistered is a factory: the listener is the closure it returns.
func mailRegistered(sent *int) func(context.Context, userRegistered) error {
	return func(context.Context, userRegistered) error {
		*sent++
		return nil
	}
}

// auditor has a listener as a method.
type auditor struct{ seen int }

func (a *auditor) onRegistered(context.Context, userRegistered) error {
	a.seen++
	return nil
}

func names(dispatcher *events.Dispatcher) []string {
	var out []string
	for _, l := range dispatcher.Listeners(userRegistered{}) {
		out = append(out, l.Name)
	}
	return out
}

func TestOnNamesTheListenerAfterItsFunction(t *testing.T) {
	dispatcher := events.New()
	defer dispatcher.Shutdown()

	var sent int
	a := &auditor{}
	events.On(dispatcher, logRegistered)
	events.On(dispatcher, mailRegistered(&sent), events.Async())
	events.ListenTo(dispatcher, a.onRegistered)

	want := []string{
		"events_test.logRegistered",           // a named function
		"events_test.mailRegistered",          // a factory's closure: .func1 dropped
		"events_test.(*auditor).onRegistered", // a method value: -fm dropped
	}
	if got := names(dispatcher); !reflect.DeepEqual(got, want) {
		t.Fatalf("names: got %q, want %q", got, want)
	}

	// The derived names are what Deliver (and so the outbox) runs.
	if err := dispatcher.Deliver(context.Background(), userRegistered{}, "events_test.mailRegistered"); err != nil || sent != 1 {
		t.Fatalf("deliver to the factory: err %v, sent %d", err, sent)
	}
	if err := dispatcher.Deliver(context.Background(), userRegistered{}, "events_test.(*auditor).onRegistered"); err != nil || a.seen != 1 {
		t.Fatalf("deliver to the method: err %v, seen %d", err, a.seen)
	}
}

func TestNameOverridesTheDerivedName(t *testing.T) {
	dispatcher := events.New()
	defer dispatcher.Shutdown()

	events.On(dispatcher, logRegistered, events.Name("user.log"))
	if got := names(dispatcher); !reflect.DeepEqual(got, []string{"user.log"}) {
		t.Fatalf("names: %q", got)
	}
}

func TestTheSameFactoryTwiceForOneEventNeedsName(t *testing.T) {
	dispatcher := events.New()
	defer dispatcher.Shutdown()

	var a, b int
	events.On(dispatcher, mailRegistered(&a))
	mustPanic(t, `"events_test.mailRegistered" is already used`, func() {
		events.On(dispatcher, mailRegistered(&b))
	})
	mustPanic(t, `"events_test.logRegistered" is already used`, func() {
		events.On(dispatcher, logRegistered)
		events.ListenTo(dispatcher, logRegistered)
	})

	// Name tells them apart.
	events.On(dispatcher, mailRegistered(&b), events.Name("mail.second"))
}

func TestAnInlineLiteralStaysUnnamed(t *testing.T) {
	dispatcher := events.New()
	defer dispatcher.Shutdown()

	// Both would be "events_test.TestAnInlineLiteralStaysUnnamed".
	events.On(dispatcher, func(context.Context, userRegistered) error { return nil })
	events.ListenTo(dispatcher, func(context.Context, userRegistered) error { return nil })
	if got := names(dispatcher); len(got) != 0 {
		t.Fatalf("inline literals must stay unnamed, got %q", got)
	}
}

func TestFuncName(t *testing.T) {
	cases := map[string]any{
		"events_test.logRegistered":           logRegistered,
		"events_test.mailRegistered":          mailRegistered(new(int)),
		"events_test.(*auditor).onRegistered": (&auditor{}).onRegistered,
		"":                                    nil,
	}
	for want, handler := range cases {
		if got := events.FuncName(handler); got != want {
			t.Errorf("FuncName: got %q, want %q", got, want)
		}
	}
	var nilFunc func(context.Context, userRegistered) error
	if got := events.FuncName(nilFunc); got != "" {
		t.Errorf("FuncName(nil func): %q", got)
	}
}

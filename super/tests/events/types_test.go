package events_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/nicklasjeppesen/going_internal/super/events"
)

// Tests for EventType: the dispatcher learns each event's Go type from its
// typed listeners.

// otherRegistered is a second type that claims userRegistered's name.
type otherRegistered struct{ Name string }

func (otherRegistered) EventName() string { return "user.registered" }

func TestEventTypeIsLearnedFromTypedListeners(t *testing.T) {
	dispatcher := events.New()
	defer dispatcher.Shutdown()

	if _, ok := dispatcher.EventType("user.registered"); ok {
		t.Fatal("a name nobody listens to has no type")
	}

	events.On(dispatcher, func(context.Context, userRegistered) error { return nil })
	events.ListenTo(dispatcher, func(context.Context, userDeleted) error { return nil })
	dispatcher.Listen("plain.name", func(context.Context, events.Event) error { return nil })

	if typ, ok := dispatcher.EventType("user.registered"); !ok || typ != reflect.TypeFor[userRegistered]() {
		t.Fatalf("On: got %v, %v", typ, ok)
	}
	if typ, ok := dispatcher.EventType("user.deleted"); !ok || typ != reflect.TypeFor[userDeleted]() {
		t.Fatalf("ListenTo: got %v, %v", typ, ok)
	}
	if _, ok := dispatcher.EventType("plain.name"); ok {
		t.Fatal("Listen doesn't know the type")
	}
}

func TestTwoTypesCannotShareAnEventName(t *testing.T) {
	dispatcher := events.New()
	defer dispatcher.Shutdown()

	events.On(dispatcher, func(context.Context, userRegistered) error { return nil })
	events.On(dispatcher, func(context.Context, userRegistered) error { return nil }) // same type: fine

	mustPanic(t, `"user.registered" is already the name of`, func() {
		events.On(dispatcher, func(context.Context, otherRegistered) error { return nil }, events.Name("other"))
	})
	if typ, _ := dispatcher.EventType("user.registered"); typ != reflect.TypeFor[userRegistered]() {
		t.Fatalf("the first type must be kept, got %v", typ)
	}
	if infos := dispatcher.Listeners(userRegistered{}); len(infos) != 0 {
		t.Fatalf("a rejected registration must not be stored: %+v", infos)
	}

	// Forget drops the type with the listeners.
	dispatcher.Forget("user.registered")
	if _, ok := dispatcher.EventType("user.registered"); ok {
		t.Fatal("Forget must drop the type")
	}
	events.On(dispatcher, func(context.Context, otherRegistered) error { return nil })
	if typ, _ := dispatcher.EventType("user.registered"); !strings.Contains(typ.String(), "otherRegistered") {
		t.Fatalf("after Forget a new type may take the name, got %v", typ)
	}
}

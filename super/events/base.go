// --------------------------------------------------------------------------
//
//	Base
//
// --------------------------------------------------------------------------
//
// The event contract of the dispatcher.
//
// A domain event is a plain description of something that already happened
// inside the domain, for example that a user was registered. An event never
// changes state by itself, it only carries the facts, and it is the listener
// that decides what should happen next. That keeps the domain code free of
// any knowledge about the side effects it triggers.
//
// Every event must implement the Event interface, and every listener is
// registered with the Dispatcher under the name the event reports.
//
// Exemple: see Event for an event and Dispatcher for the registration.
package events

import (
	"context"
	"errors"
)

// Wildcard is the reserved listener name that matches every published event.
const Wildcard = "*"

// ErrShutdown is returned when an event is published to a dispatcher that has
// already been shut down.
var ErrShutdown = errors.New("events: dispatcher is shutting down")

// ErrNoListener is wrapped by the error Deliver returns when the event has no
// listener with the given name.
var ErrNoListener = errors.New("events: no listener with that name")

// Event is the contract every domain event must fulfill.
//
// The name is the routing key the dispatcher uses to find the listeners, so
// it has to stay stable once listeners are written against it.
//
// Exemple:
//
//	type UserRegistered struct {
//		Email string
//	}
//
//	func (UserRegistered) EventName() string {
//		return "user.registered"
//	}
type Event interface {

	// EventName returns the routing key of the event.
	EventName() string
}

// Handler reacts to a published event.
//
// A handler that returns an error never stops the other listeners, the
// dispatcher keeps going and collects the error instead.
type Handler func(ctx context.Context, event Event) error

// Base is embedded by every domain event, so an event only has to describe
// its name and can leave the plumbing to this struct.
//
//	type UserRegistered struct {
//		events.Base
//		Email string
//	}
//
//	func (UserRegistered) EventName() string {
//		return "user.registered"
//	}
type Base struct {
}

// EventName returns the routing key of the event.
//
// Embedding types override it by declaring their own EventName method.
func (Base) EventName() string {
	return BaseEventName
}

// BaseEventName is the fallback name of an event that does not name itself.
const BaseEventName = "events.base"

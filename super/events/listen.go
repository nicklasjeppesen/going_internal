package events

import (
	"context"
	"fmt"
)

// mode says how a listener wants to be called.
type mode int

const (
	// modeFollowCaller is the mode of Listen: the listener runs on the worker
	// pool for Publish and inline for Dispatch.
	modeFollowCaller mode = iota

	// modeSync runs the listener in the goroutine that publishes the event,
	// before Publish returns.
	modeSync

	// modeAsync runs the listener on the worker pool.
	modeAsync
)

// registration is a handler together with the way it wants to be called.
type registration struct {
	handler Handler
	mode    mode
}

// Filters for handlersFor.
func everyMode(mode) bool  { return true }
func onlySync(m mode) bool { return m == modeSync }
func skipSync(m mode) bool { return m != modeSync }

// ListenOption customises a listener registered with On.
type ListenOption func(*registration)

// Async runs the listener on the worker pool, so Publish does not wait for it.
//
// Use it for side effects that are slow or talk to the outside world, such as
// sending a mail or pushing to a websocket.
func Async() ListenOption {
	return func(entry *registration) {
		entry.mode = modeAsync
	}
}

// On registers a listener that receives the event as its own type, and that
// decides itself how it is called.
//
// Without options the listener is synchronous: it runs before Publish returns,
// and its error is returned by Publish. Pass Async to run it on the worker
// pool instead. The one publishing the event never has to know which is which.
//
//	events.On(dispatcher, listeners.ConfirmOrder)                  // sync
//	events.On(dispatcher, listeners.NotifyCustomer, events.Async()) // worker pool
//
// Dispatch still runs every listener inline, whatever its mode.
func On[T Event](dispatcher *Dispatcher, handler func(ctx context.Context, event T) error, options ...ListenOption) {
	var prototype T
	var name = prototype.EventName()

	var entry = registration{
		mode: modeSync,
		handler: func(ctx context.Context, event Event) error {
			typed, ok := event.(T)
			if !ok {
				return fmt.Errorf("listener for %q expected %T, received %T", name, prototype, event)
			}

			return handler(ctx, typed)
		},
	}

	for _, option := range options {
		option(&entry)
	}

	dispatcher.register(name, entry)
}

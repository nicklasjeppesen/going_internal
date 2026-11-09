package events

import (
	"context"
	"errors"
)

// Publisher is what controllers and commands publish domain events with.
// Dispatcher fulfils it; in a test a fake Publisher can collect the events
// instead of running listeners.
type Publisher interface {
	PublishAll(ctx context.Context, events ...Event) error
}

// Recorder is a model that has recorded events about what happened to it.
// Every ActiveRecord model in super/db is one (see ActiveRecord.Record).
// ReleaseEvents returns the events and forgets them, so they are only
// published once.
type Recorder interface {
	ReleaseEvents() []Event
}

// PublishAll publishes the events in order, after the action has succeeded.
// Each goes through PublishDetached: the context keeps its values (such as
// the logged-in user) but is never cancelled, so the async listeners can
// finish after the response has been sent. The sync listeners' errors are
// joined and returned; a failing event never stops the ones after it.
//
//	if err := todos.Save(ctx, todo); err != nil {
//		return err
//	}
//	return publisher.PublishAll(ctx, todo.ReleaseEvents()...)
func (dispatcher *Dispatcher) PublishAll(ctx context.Context, events ...Event) error {
	var errs []error
	for _, event := range events {
		if err := dispatcher.PublishDetached(ctx, event); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

var _ Publisher = (*Dispatcher)(nil)

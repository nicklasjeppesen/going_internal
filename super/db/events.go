package db

import (
	"slices"

	"github.com/nicklasjeppesen/going_internal/super/events"
)

// Domain events on a model.
//
// Every ActiveRecord model can record domain events about what happened to
// it, so a business method can say what happened without publishing anything:
//
//	func (t *Todo) Complete(by int64) bool {
//		if t.Completed_at != "" {
//			return false
//		}
//		t.Completed_at = now()
//		t.Record(TodoCompleted{TodoID: t.Id, By: by})
//		return true
//	}
//
// The ORM never publishes by itself, not on Save and not on Update. The code
// that saved the model publishes what it recorded, so a listener never reacts
// to something that wasn't stored:
//
//	if err := todo.Update(); err != nil {
//		return err
//	}
//	return publisher.PublishAll(ctx, todo.ReleaseEvents()...)
//
// The events live in an unexported field, so they are never written as a
// column and never included in ToJson. A model that copies its receiver in
// DB(ctx) keeps them; rows loaded with First or Get start without any.
// Every model is therefore an events.Recorder.

// Record records that event happened to the model. It is published later by
// whoever saves the model, with ReleaseEvents.
func (activerecord *ActiveRecord[T]) Record(event events.Event) {
	// Clip, so a copy of the model (DB copies its receiver) never appends
	// into the same backing array as the original.
	activerecord.recordedEvents = append(slices.Clip(activerecord.recordedEvents), event)
}

// ReleaseEvents returns the recorded events in the order they were recorded
// and forgets them, so they are only published once.
func (activerecord *ActiveRecord[T]) ReleaseEvents() []events.Event {
	out := activerecord.recordedEvents
	activerecord.recordedEvents = nil
	return out
}

// forgetEvents drops the recorded events without returning them.
func (activerecord *ActiveRecord[T]) forgetEvents() {
	activerecord.recordedEvents = nil
}

// eventForgetter is a model that embeds ActiveRecord.
type eventForgetter interface{ forgetEvents() }

// loadedRow makes a row that was just read from the database start without
// recorded events, also when it was made from a model that had some.
func loadedRow(row any) {
	if forgetter, ok := row.(eventForgetter); ok {
		forgetter.forgetEvents()
	}
}

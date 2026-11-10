package outbox_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/nicklasjeppesen/going_internal/super/customrouter"
	"github.com/nicklasjeppesen/going_internal/super/events"
	"github.com/nicklasjeppesen/going_internal/super/events/outbox"
)

// counter is a listener factory's dependency.
type counter interface{ Add() }

type atomicCounter struct{ n atomic.Int32 }

func (c *atomicCounter) Add() { c.n.Add(1) }

// CountTaskDone is a listener factory.
func CountTaskDone(c counter) func(context.Context, taskDone) error {
	return func(context.Context, taskDone) error { c.Add(); return nil }
}

func TestTheOutboxDeliversToFactoryListeners(t *testing.T) {
	database, _, _, ctx := setup(t)

	c := &atomicCounter{}
	container := customrouter.NewContainer().Register(func() counter { return c })
	dispatcher := events.New(events.WithResolver(container))
	t.Cleanup(dispatcher.Shutdown)
	relay := outbox.NewRelay(dispatcher)

	events.On(dispatcher, CountTaskDone)
	events.On(dispatcher, CountTaskDone, events.Async(), events.Name("count.async"))

	if err := relay.PublishAll(ctx, taskDone{ID: 1}); err != nil {
		t.Fatal(err)
	}
	relay.Wait()

	d := deliveries(t, database)
	if len(d) != 2 || d["outbox_test.CountTaskDone"].delivered == "" || d["count.async"].delivered == "" {
		t.Fatalf("deliveries: %+v", d)
	}
	if c.n.Load() != 2 {
		t.Fatalf("the factory's listeners ran %d times", c.n.Load())
	}
}

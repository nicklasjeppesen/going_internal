package events_test

import (
	"context"
	"errors"
	"testing"

	"github.com/nicklasjeppesen/going_internal/super/events"
)

func TestPublishAllPublishesEveryEventInOrder(t *testing.T) {
	dispatcher := events.New()
	defer dispatcher.Shutdown()

	var got []string
	events.On(dispatcher, func(_ context.Context, event userRegistered) error {
		got = append(got, event.Email)
		return nil
	})
	events.On(dispatcher, func(_ context.Context, event userDeleted) error {
		got = append(got, "deleted")
		return nil
	})

	var publisher events.Publisher = dispatcher
	err := publisher.PublishAll(context.Background(),
		userRegistered{Email: "a@b.dk"}, userDeleted{ID: 1}, userRegistered{Email: "c@d.dk"})
	if err != nil {
		t.Fatalf("PublishAll returned %v, want nil", err)
	}

	if want := []string{"a@b.dk", "deleted", "c@d.dk"}; len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("PublishAll: got %v, want %v", got, want)
	}
}

func TestPublishAllWithNoEventsDoesNothing(t *testing.T) {
	dispatcher := events.New()
	defer dispatcher.Shutdown()

	if err := dispatcher.PublishAll(context.Background()); err != nil {
		t.Fatalf("PublishAll() returned %v, want nil", err)
	}
}

func TestPublishAllJoinsErrorsAndKeepsGoing(t *testing.T) {
	dispatcher := events.New()
	defer dispatcher.Shutdown()

	var errBoom = errors.New("boom")
	var deleted bool
	events.On(dispatcher, func(context.Context, userRegistered) error { return errBoom })
	events.On(dispatcher, func(context.Context, userDeleted) error {
		deleted = true
		return nil
	})

	err := dispatcher.PublishAll(context.Background(), userRegistered{}, userDeleted{})
	if !errors.Is(err, errBoom) {
		t.Fatalf("PublishAll returned %v, want it to wrap %v", err, errBoom)
	}
	if !deleted {
		t.Error("a failing event must not stop the events after it")
	}
}

func TestPublishAllSurvivesACancelledCaller(t *testing.T) {
	dispatcher := events.New()

	var called = make(chan struct{}, 1)
	events.On(dispatcher, func(ctx context.Context, _ userRegistered) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		called <- struct{}{}
		return nil
	}, events.Async())

	ctx, cancel := context.WithCancel(context.Background())
	if err := dispatcher.PublishAll(ctx, userRegistered{}); err != nil {
		t.Fatal(err)
	}
	cancel() // the request is over before the worker runs

	dispatcher.Shutdown() // drains the queue
	select {
	case <-called:
	default:
		t.Fatal("the async listener did not run after the caller was cancelled")
	}
}

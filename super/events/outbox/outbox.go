// Package outbox is a transactional outbox for super/events, with one delivery
// per named listener (like one job per side effect):
//
//  1. Relay.Store saves domain events in domain_events. Called with a ctx from
//     db.Transaction, the events are saved in the same transaction as the
//     change they are about, so an event is never lost once the change is
//     stored, and never stored for a change that rolled back.
//  2. The relay fans each stored event out to one row in event_deliveries per
//     named listener (events.Dispatcher.Listeners) and marks it published.
//  3. Each delivery runs on its own (events.Dispatcher.Deliver): sync ones in
//     the caller's goroutine, async ones in the background. When one fails,
//     only that one is retried (Relay.Retry, every 30 seconds from Relay.Job),
//     not the other listeners of the event.
//
// A sync delivery runs its listener and marks itself delivered in one
// db.Transaction: what the listener writes through the ORM or db.Conn(ctx)
// and "delivered" commit together or not at all, so a listener that only
// changes the database runs exactly once. What leaves the process (a mail, a
// push from an async listener) can't be part of a transaction and is
// delivered at least once.
//
// Only named listeners are delivered by the outbox: those registered with On
// or ListenTo (named after their function, or with events.Name) and those given
// events.Name. Unnamed ones (Listen, inline func literals without Name) never
// run for events that go through it.
//
// The tables are in Schema (SQLite and Postgres).
package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"reflect"
	"sync"
	"time"

	"github.com/nicklasjeppesen/going_internal/super/db"
	"github.com/nicklasjeppesen/going_internal/super/events"
	"github.com/nicklasjeppesen/going_internal/super/jobs"
)

// MaxAttempts is how many times an event is fanned out or a delivery is tried;
// after that it stays with its last error (last_error).
const MaxAttempts = 10

// StaleClaim is how long an event or a delivery may stay claimed by a process
// before Retry frees it again (the process probably died).
const StaleClaim = time.Minute

// RetryInterval is how often Job retries what is still pending.
const RetryInterval = 30 * time.Second

// Relay stores, fans out and delivers the outbox's events. It fulfils
// events.Publisher, so commands publish through it.
type Relay struct {
	dispatcher *events.Dispatcher
	types      map[string]reflect.Type
	typesMu    sync.RWMutex
	background sync.WaitGroup // async deliveries that are running

	// actor and restore are set by WithActor.
	actor   func(ctx context.Context) int64
	restore func(ctx context.Context, id int64) (context.Context, error)
}

// Option customises a Relay.
type Option func(*Relay)

// WithActor stores who caused an event (actor reads the id from the context
// Store gets, 0 = nobody) and runs its listeners as them (restore puts that
// id back into the delivery's context, for example by loading the user).
// restore is only called when the context doesn't already hold that id.
// Without WithActor no actor is stored (user_id stays 0).
func WithActor(actor func(ctx context.Context) int64, restore func(ctx context.Context, id int64) (context.Context, error)) Option {
	return func(relay *Relay) {
		relay.actor, relay.restore = actor, restore
	}
}

// NewRelay makes a relay that delivers with dispatcher. The outbox's tables
// live on the default connection (db.Pool).
func NewRelay(dispatcher *events.Dispatcher, options ...Option) *Relay {
	relay := &Relay{dispatcher: dispatcher, types: map[string]reflect.Type{}}
	for _, option := range options {
		option(relay)
	}
	return relay
}

// Know tells the relay the Go types of events it reads back from the database:
// one value of each type, e.g. TodoCompleted{}. It is normally not needed: the
// relay uses the type the dispatcher learned when a listener was registered
// with On or ListenTo. Know is for an event whose listeners don't name its
// type (Listen, or a named ListenAll), and it wins over the dispatcher.
//
// An event is stored as JSON, so its fields must survive json.Marshal and
// json.Unmarshal.
func (r *Relay) Know(prototypes ...events.Event) {
	r.typesMu.Lock()
	defer r.typesMu.Unlock()
	for _, p := range prototypes {
		r.types[p.EventName()] = reflect.TypeOf(p)
	}
}

// Store saves events in domain_events through the ORM with ctx: inside
// db.Transaction they join its transaction. The actor (WithActor) is saved
// with them.
func (r *Relay) Store(ctx context.Context, list ...events.Event) error {
	var actor int64
	if r.actor != nil {
		actor = r.actor(ctx)
	}
	for _, e := range list {
		payload, err := json.Marshal(e)
		if err != nil {
			return fmt.Errorf("outbox: %q: %w", e.EventName(), err)
		}
		row := eventRow{Name: e.EventName(), Payload: string(payload), User_id: actor}.DB(ctx)
		if _, err := row.Save(); err != nil {
			return fmt.Errorf("outbox: store %q: %w", e.EventName(), err)
		}
	}
	return nil
}

// PublishAll stores events and then flushes what is pending: the sync
// deliveries now, the async ones in the background. Only an error from
// storing is returned; a delivery that fails is retried by Retry.
//
// Called inside db.Transaction, the events are stored in the transaction and
// flushed only after it has committed (db.AfterCommit); after a rollback
// nothing is stored or delivered.
func (r *Relay) PublishAll(ctx context.Context, list ...events.Event) error {
	if err := r.Store(ctx, list...); err != nil {
		return err
	}
	flushCtx := db.WithoutTx(ctx)
	db.AfterCommit(ctx, func() {
		if err := r.Flush(flushCtx); err != nil {
			log.Printf("outbox: %v", err)
		}
	})
	return nil
}

// Flush fans out the pending events and delivers the pending deliveries: the
// sync ones in the calling goroutine, the async ones in the background (Wait
// waits for them).
func (r *Relay) Flush(ctx context.Context) error {
	ctx = db.WithoutTx(ctx)
	return errors.Join(r.fanOut(ctx), r.deliverPending(ctx, false))
}

// Retry frees the events and deliveries a process claimed more than
// StaleClaim ago without finishing, and fans out and delivers everything that
// is pending, all in the calling goroutine.
func (r *Relay) Retry(ctx context.Context) error {
	ctx = db.WithoutTx(ctx)
	stale := stamp(time.Now().Add(-StaleClaim))
	for _, table := range []string{"domain_events", "event_deliveries"} {
		if _, err := db.Pool().ExecContext(ctx, db.Rebind(`UPDATE `+table+` SET claimed_at = '' WHERE claimed_at != '' AND claimed_at < ?`), stale); err != nil {
			return fmt.Errorf("outbox: free stale claims: %w", err)
		}
	}
	return errors.Join(r.fanOut(ctx), r.deliverPending(ctx, true))
}

// Wait waits until the async deliveries Flush started are done.
func (r *Relay) Wait() { r.background.Wait() }

// Job is a job that calls Retry every RetryInterval. Register it with the
// app's scheduler.
func (r *Relay) Job() jobs.Job {
	return jobs.Job{
		Title:          "Outbox: retry pending domain events",
		Interval:       RetryInterval,
		TerminateAfter: RetryInterval - 5*time.Second,
		Runner: func(job jobs.Job) {
			ctx := job.Ctx()
			if ctx == nil {
				ctx = context.Background()
			}
			if err := r.Retry(ctx); err != nil {
				log.Printf("outbox: %v", err)
			}
		},
	}
}

var _ events.Publisher = (*Relay)(nil)

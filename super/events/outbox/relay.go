package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"reflect"
	"time"

	"github.com/nicklasjeppesen/going_internal/super/db"
	"github.com/nicklasjeppesen/going_internal/super/events"
)

// storedEvent is an event read back from domain_events.
type storedEvent struct {
	id      int64
	name    string
	payload string
	userID  int64
}

// delivery is one event for one named listener, from event_deliveries.
type delivery struct {
	id       int64
	listener string
	async    bool
	event    storedEvent
}

// fanOut gives every pending event one delivery per named listener and marks
// it published. An event without named listeners is just marked published.
func (r *Relay) fanOut(ctx context.Context) error {
	pending, err := r.pendingEvents(ctx)
	if err != nil {
		return err
	}
	var errs []error
	for _, e := range pending {
		if !claim(ctx, "domain_events", e.id) {
			continue
		}
		if err := r.distribute(ctx, e); err != nil {
			errs = append(errs, err)
			failed(ctx, "domain_events", e.id, err)
		}
	}
	return errors.Join(errs...)
}

// distribute makes the event's deliveries and marks it published, in one
// transaction. A listener that already has a delivery for the event is
// skipped, so a second fan-out never doubles one. An event without named
// listeners is just marked published, also when its type is unknown; with
// listeners, the event must decode (so a delivery can run it).
func (r *Relay) distribute(ctx context.Context, e storedEvent) error {
	listeners := r.dispatcher.Listeners(eventName(e.name))
	if len(listeners) > 0 {
		if _, err := r.decode(e); err != nil {
			return err
		}
	}
	return db.Transaction(ctx, func(ctx context.Context) error {
		existing, err := deliveredListeners(ctx, e.id)
		if err != nil {
			return err
		}
		for _, l := range listeners {
			if existing[l.Name] {
				continue
			}
			row := deliveryRow{Event_id: e.id, Listener: l.Name, Async: boolInt(l.Async)}.DB(ctx)
			if _, err := row.Save(); err != nil {
				return fmt.Errorf("outbox: delivery of event %d to %q: %w", e.id, l.Name, err)
			}
		}
		now := stamp(time.Now())
		_, err = db.Conn(ctx).ExecContext(ctx, db.Rebind(`UPDATE domain_events SET published_at = ?, claimed_at = '',
			attempts = attempts + 1, last_error = '', updated_at = ? WHERE id = ?`), now, now, e.id)
		return err
	})
}

// deliveredListeners is the set of listeners that already have a delivery for
// the event id.
func deliveredListeners(ctx context.Context, id int64) (map[string]bool, error) {
	rows, err := db.Conn(ctx).QueryContext(ctx, db.Rebind(`SELECT listener FROM event_deliveries WHERE event_id = ?`), id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out[name] = true
	}
	return out, rows.Err()
}

// deliverPending runs the pending deliveries. With inline every one runs in
// the calling goroutine; otherwise the async ones run in the background.
func (r *Relay) deliverPending(ctx context.Context, inline bool) error {
	pending, err := pendingDeliveries(ctx)
	if err != nil {
		return err
	}
	var errs []error
	for _, d := range pending {
		if !claim(ctx, "event_deliveries", d.id) {
			continue
		}
		if d.async && !inline {
			r.background.Add(1)
			go func() {
				defer r.background.Done()
				if err := r.deliver(context.WithoutCancel(ctx), d); err != nil {
					log.Printf("outbox: %v", err)
				}
			}()
			continue
		}
		if err := r.deliver(ctx, d); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// deliver runs the delivery d as the event's actor and marks it delivered, or
// frees it with its error for the next try. A sync delivery runs in one
// transaction with its mark (deliverInTx). A listener that no longer exists
// (events.ErrNoListener) is given up.
func (r *Relay) deliver(ctx context.Context, d delivery) error {
	var err error
	if d.async {
		if err = r.run(ctx, d); err == nil {
			err = markDelivered(ctx, d.id, "")
		}
	} else {
		err = r.deliverInTx(ctx, d)
	}
	switch {
	case err == nil:
		return nil
	case errors.Is(err, events.ErrNoListener):
		if markErr := markDelivered(ctx, d.id, "given up: "+err.Error()); markErr != nil {
			return markErr
		}
		return nil
	}
	failed(ctx, "event_deliveries", d.id, err)
	return fmt.Errorf("outbox: delivery %d (%s): %w", d.id, d.listener, err)
}

// deliverInTx runs the listener and the mark "delivered" in one transaction,
// which the listener joins through its ctx. When the listener fails,
// everything it wrote rolls back.
func (r *Relay) deliverInTx(ctx context.Context, d delivery) error {
	return db.Transaction(ctx, func(ctx context.Context) error {
		if err := r.run(ctx, d); err != nil {
			return err
		}
		return markDelivered(ctx, d.id, "")
	})
}

// markDelivered marks the delivery id delivered (lastError says why, when it
// was given up).
func markDelivered(ctx context.Context, id int64, lastError string) error {
	now := stamp(time.Now())
	_, err := db.Conn(ctx).ExecContext(ctx, db.Rebind(`UPDATE event_deliveries SET delivered_at = ?, claimed_at = '',
		attempts = attempts + 1, last_error = ?, updated_at = ? WHERE id = ?`), now, lastError, now, id)
	return err
}

// run runs one listener for the delivery's event.
func (r *Relay) run(ctx context.Context, d delivery) error {
	event, err := r.decode(d.event)
	if err != nil {
		return err
	}
	ctx, err = r.asActor(ctx, d.event.userID)
	if err != nil {
		return err
	}
	return r.dispatcher.Deliver(ctx, event, d.listener)
}

// pendingEvents are the events waiting to be fanned out.
func (r *Relay) pendingEvents(ctx context.Context) ([]storedEvent, error) {
	rows, err := db.Pool().QueryContext(ctx, db.Rebind(`SELECT id, name, payload, user_id FROM domain_events
		WHERE published_at = '' AND claimed_at = '' AND attempts < ? ORDER BY id LIMIT 100`), MaxAttempts)
	if err != nil {
		return nil, fmt.Errorf("outbox: pending events: %w", err)
	}
	defer rows.Close()
	var out []storedEvent
	for rows.Next() {
		var e storedEvent
		if err := rows.Scan(&e.id, &e.name, &e.payload, &e.userID); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// pendingDeliveries are the deliveries waiting to run, with their event.
func pendingDeliveries(ctx context.Context) ([]delivery, error) {
	rows, err := db.Pool().QueryContext(ctx, db.Rebind(`SELECT d.id, d.listener, d.async, e.id, e.name, e.payload, e.user_id
		FROM event_deliveries d JOIN domain_events e ON e.id = d.event_id
		WHERE d.delivered_at = '' AND d.claimed_at = '' AND d.attempts < ? ORDER BY d.id LIMIT 200`), MaxAttempts)
	if err != nil {
		return nil, fmt.Errorf("outbox: pending deliveries: %w", err)
	}
	defer rows.Close()
	var out []delivery
	for rows.Next() {
		var d delivery
		var async int64
		if err := rows.Scan(&d.id, &d.listener, &async, &d.event.id, &d.event.name, &d.event.payload, &d.event.userID); err != nil {
			return nil, err
		}
		d.async = async == 1
		out = append(out, d)
	}
	return out, rows.Err()
}

// claim takes the row id in table, so no other process works on it at the
// same time. It is a conditional UPDATE: only the process that changed the
// row has it.
func claim(ctx context.Context, table string, id int64) bool {
	res, err := db.Pool().ExecContext(ctx, db.Rebind(`UPDATE `+table+` SET claimed_at = ? WHERE id = ? AND claimed_at = ''`), stamp(time.Now()), id)
	if err != nil {
		return false
	}
	n, _ := res.RowsAffected()
	return n == 1
}

// failed frees the row id in table with err for the next try.
func failed(ctx context.Context, table string, id int64, err error) {
	if _, dbErr := db.Pool().ExecContext(ctx, db.Rebind(`UPDATE `+table+` SET claimed_at = '', attempts = attempts + 1,
		last_error = ?, updated_at = ? WHERE id = ?`), err.Error(), stamp(time.Now()), id); dbErr != nil {
		log.Printf("outbox: record the failure of %s %d: %v", table, id, dbErr)
	}
}

// eventName is an event known only by its name, for asking the dispatcher
// about its listeners without decoding it.
type eventName string

func (name eventName) EventName() string { return string(name) }

// eventType is the Go type of the event name: the one given to Know, or else
// the one the dispatcher learned from a typed listener (On, ListenTo).
func (r *Relay) eventType(name string) (reflect.Type, bool) {
	r.typesMu.RLock()
	typ, ok := r.types[name]
	r.typesMu.RUnlock()
	if ok {
		return typ, true
	}
	return r.dispatcher.EventType(name)
}

// decode reads the event back as its own type.
func (r *Relay) decode(e storedEvent) (events.Event, error) {
	typ, ok := r.eventType(e.name)
	if !ok {
		return nil, fmt.Errorf("outbox: unknown event %q (no typed listener registered with On/ListenTo, and not given to Know)", e.name)
	}
	value := reflect.New(typ)
	if err := json.Unmarshal([]byte(e.payload), value.Interface()); err != nil {
		return nil, fmt.Errorf("outbox: %q: %w", e.name, err)
	}
	return value.Elem().Interface().(events.Event), nil
}

// asActor is ctx running as the actor id (see WithActor).
func (r *Relay) asActor(ctx context.Context, id int64) (context.Context, error) {
	if id == 0 || r.restore == nil || (r.actor != nil && r.actor(ctx) == id) {
		return ctx, nil
	}
	return r.restore(ctx, id)
}

// stamp is a time as the outbox stores it: UTC, sortable as text.
func stamp(t time.Time) string { return t.UTC().Format("2006-01-02 15:04:05") }

// boolInt is 1 for true and 0 for false (the async column is an INTEGER).
func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

package outbox_test

import (
	"context"
	"errors"
	"os"
	"sync/atomic"
	"testing"

	"github.com/nicklasjeppesen/going_internal/super/db"
	"github.com/nicklasjeppesen/going_internal/super/events"
	"github.com/nicklasjeppesen/going_internal/super/events/outbox"
	. "github.com/nicklasjeppesen/going_internal/super/tests/db/shared"
	model "github.com/nicklasjeppesen/going_internal/super/tests/db/shared/models"
	schema "github.com/nicklasjeppesen/going_internal/super/tests/db/shared/schema"
)

// Tests for the transactional outbox: Store in a transaction, fan-out to one
// delivery per named listener, retries per listener, and sync deliveries that
// commit together with what their listener wrote.

// taskDone is the event the tests publish.
type taskDone struct {
	ID    int64
	Title string
}

func (taskDone) EventName() string { return "task.done" }

// actorKey is where the tests keep the actor in a context.
type actorKey struct{}

func actorOf(ctx context.Context) int64 {
	id, _ := ctx.Value(actorKey{}).(int64)
	return id
}

var errFlaky = errors.New("temporary failure")

// setup makes a database with the outbox's tables and the shared users table,
// and a relay that stores the actor and restores it.
func setup(t *testing.T) (*TestDB, *events.Dispatcher, *outbox.Relay, context.Context) {
	t.Helper()
	database, err := NewInMemoryDB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		database.DB.Close()
		for _, f := range []string{"testdb.db", "testdb.db-wal", "testdb.db-shm"} {
			os.Remove(f)
		}
	})
	for _, s := range []string{schema.UserSchema, outbox.SQLiteSchema} {
		if err := database.LoadSchema(s); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("DB_CONNECTION", "sqlite")
	t.Setenv("DB_PATH", "./testdb.db")

	dispatcher := events.New()
	t.Cleanup(dispatcher.Shutdown)
	relay := outbox.NewRelay(dispatcher, outbox.WithActor(actorOf, func(ctx context.Context, id int64) (context.Context, error) {
		return context.WithValue(ctx, actorKey{}, id), nil
	}))
	// No Know: the relay learns taskDone from the typed listeners (On).
	return database, dispatcher, relay, context.WithValue(context.Background(), actorKey{}, int64(7))
}

// eventRow is a row in domain_events.
type eventRow struct {
	name, published, lastError string
	userID                     int64
	attempts                   int
}

func eventRows(t *testing.T, database *TestDB) []eventRow {
	t.Helper()
	rows, err := database.DB.Query(`SELECT name, published_at, last_error, user_id, attempts FROM domain_events ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []eventRow
	for rows.Next() {
		var r eventRow
		rows.Scan(&r.name, &r.published, &r.lastError, &r.userID, &r.attempts)
		out = append(out, r)
	}
	return out
}

// deliveryRow is a row in event_deliveries.
type deliveryRow struct {
	delivered, lastError string
	async, attempts      int
}

func deliveries(t *testing.T, database *TestDB) map[string]deliveryRow {
	t.Helper()
	rows, err := database.DB.Query(`SELECT listener, async, delivered_at, last_error, attempts FROM event_deliveries ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]deliveryRow{}
	for rows.Next() {
		var name string
		var r deliveryRow
		rows.Scan(&name, &r.async, &r.delivered, &r.lastError, &r.attempts)
		out[name] = r
	}
	return out
}

func users(t *testing.T, database *TestDB, name string) int {
	t.Helper()
	var n int
	database.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE name = ?`, name).Scan(&n)
	return n
}

func TestStoreFollowsTheTransaction(t *testing.T) {
	database, _, relay, ctx := setup(t)

	err := db.Transaction(ctx, func(ctx context.Context) error {
		if err := relay.Store(ctx, taskDone{ID: 1}); err != nil {
			return err
		}
		return errFlaky
	})
	if !errors.Is(err, errFlaky) {
		t.Fatal(err)
	}
	if rows := eventRows(t, database); len(rows) != 0 {
		t.Fatalf("an event from a rolled back transaction was stored: %+v", rows)
	}

	if err := db.Transaction(ctx, func(ctx context.Context) error { return relay.Store(ctx, taskDone{ID: 2}) }); err != nil {
		t.Fatal(err)
	}
	if rows := eventRows(t, database); len(rows) != 1 || rows[0].name != "task.done" || rows[0].userID != 7 || rows[0].published != "" {
		t.Fatalf("a committed event: %+v", rows)
	}
}

func TestFanOutAndDelivery(t *testing.T) {
	database, dispatcher, relay, ctx := setup(t)

	var heard []taskDone
	var asyncActor atomic.Int64
	var unnamed atomic.Int32
	events.On(dispatcher, func(_ context.Context, e taskDone) error { heard = append(heard, e); return nil }, events.Name("log"))
	events.On(dispatcher, func(ctx context.Context, _ taskDone) error { asyncActor.Store(actorOf(ctx)); return nil },
		events.Async(), events.Name("push"))
	events.On(dispatcher, func(context.Context, taskDone) error { unnamed.Add(1); return nil })

	// The async listener runs as the stored actor, also from a context without one.
	if err := relay.PublishAll(ctx, taskDone{ID: 5, Title: "Lagkage"}); err != nil {
		t.Fatal(err)
	}
	relay.Wait()

	if rows := eventRows(t, database); rows[0].published == "" || rows[0].attempts != 1 {
		t.Fatalf("the event must be fanned out: %+v", rows)
	}
	d := deliveries(t, database)
	if len(d) != 2 || d["log"].delivered == "" || d["log"].async != 0 || d["push"].delivered == "" || d["push"].async != 1 {
		t.Fatalf("deliveries: %+v", d)
	}
	if len(heard) != 1 || heard[0] != (taskDone{ID: 5, Title: "Lagkage"}) {
		t.Fatalf("heard: %+v", heard)
	}
	if asyncActor.Load() != 7 {
		t.Fatalf("the async listener ran as %d, want the actor 7", asyncActor.Load())
	}
	if unnamed.Load() != 0 {
		t.Fatal("the outbox delivers only named listeners")
	}
}

func TestOnlyTheFailingListenerIsRetried(t *testing.T) {
	database, dispatcher, relay, ctx := setup(t)

	var fail atomic.Bool
	fail.Store(true)
	var flaky, steady atomic.Int32
	events.On(dispatcher, func(context.Context, taskDone) error {
		flaky.Add(1)
		if fail.Load() {
			return errFlaky
		}
		return nil
	}, events.Name("flaky"))
	events.On(dispatcher, func(context.Context, taskDone) error { steady.Add(1); return nil }, events.Async(), events.Name("steady"))

	if err := relay.PublishAll(ctx, taskDone{ID: 1}); err != nil {
		t.Fatalf("a failing listener must not fail PublishAll: %v", err)
	}
	relay.Wait()
	d := deliveries(t, database)
	if d["flaky"].delivered != "" || d["flaky"].attempts != 1 || d["flaky"].lastError == "" || d["steady"].delivered == "" {
		t.Fatalf("after the first try: %+v", d)
	}

	fail.Store(false)
	if err := relay.Retry(ctx); err != nil {
		t.Fatal(err)
	}
	if d := deliveries(t, database)["flaky"]; d.delivered == "" || d.attempts != 2 || d.lastError != "" {
		t.Fatalf("after the retry: %+v", d)
	}
	if flaky.Load() != 2 || steady.Load() != 1 {
		t.Fatalf("flaky ran %d times (want 2), steady %d (want 1)", flaky.Load(), steady.Load())
	}
}

func TestSyncListenerThatWritesThenFailsLeavesNothing(t *testing.T) {
	database, dispatcher, relay, ctx := setup(t)

	var fail atomic.Bool
	fail.Store(true)
	events.On(dispatcher, func(ctx context.Context, e taskDone) error {
		user := model.User{}.DB(ctx) // joins the delivery's transaction through ctx
		user.Name = "written by the listener"
		if _, err := user.Save(); err != nil {
			return err
		}
		if fail.Load() {
			return errFlaky
		}
		return nil
	}, events.Name("writes"))

	if err := relay.PublishAll(ctx, taskDone{ID: 1}); err != nil {
		t.Fatal(err)
	}
	if n := users(t, database, "written by the listener"); n != 0 {
		t.Fatalf("a failed delivery must not leave what it wrote, found %d", n)
	}
	if d := deliveries(t, database)["writes"]; d.delivered != "" || d.attempts != 1 {
		t.Fatalf("after the failure: %+v", d)
	}

	fail.Store(false)
	if err := relay.Retry(ctx); err != nil {
		t.Fatal(err)
	}
	if n := users(t, database, "written by the listener"); n != 1 {
		t.Fatalf("the retry must write exactly once, found %d", n)
	}
}

// An event with no named listeners is just marked published, also when no
// type is known for it.
func TestAnUnknownEventWithoutListenersIsPublished(t *testing.T) {
	database, _, relay, ctx := setup(t)
	database.DB.Exec(`INSERT INTO domain_events (name, payload, user_id) VALUES ('unknown.event', '{}', 1)`)

	if err := relay.Flush(ctx); err != nil {
		t.Fatalf("an event nobody listens to must not fail: %v", err)
	}
	if rows := eventRows(t, database); rows[0].published == "" || rows[0].lastError != "" {
		t.Fatalf("unknown event without listeners: %+v", rows)
	}
	if d := deliveries(t, database); len(d) != 0 {
		t.Fatalf("no deliveries for it: %+v", d)
	}
}

// An event with listeners must decode: without a known type it stays with
// its error.
func TestAnUnknownEventWithListenersFails(t *testing.T) {
	database, dispatcher, relay, ctx := setup(t)
	// A named wildcard listener hears every event, but doesn't name a type.
	dispatcher.ListenAll(func(context.Context, events.Event) error { return nil }, events.Name("audit"))
	database.DB.Exec(`INSERT INTO domain_events (name, payload, user_id) VALUES ('unknown.event', '{}', 1)`)

	if err := relay.Flush(ctx); err == nil {
		t.Fatal("an unknown event with listeners must give an error")
	}
	if rows := eventRows(t, database); rows[0].published != "" || rows[0].lastError == "" || rows[0].attempts != 1 {
		t.Fatalf("unknown event with listeners: %+v", rows)
	}
}

// Know gives the type of an event whose listeners don't name it.
func TestKnowDecodesAnEventOnlyAWildcardListenerHears(t *testing.T) {
	_, dispatcher, relay, ctx := setup(t)
	var heard []events.Event
	dispatcher.ListenAll(func(_ context.Context, e events.Event) error { heard = append(heard, e); return nil }, events.Name("audit"))
	relay.Know(auditOnly{})

	if err := relay.PublishAll(ctx, auditOnly{Note: "hej"}); err != nil {
		t.Fatal(err)
	}
	if len(heard) != 1 || heard[0] != (auditOnly{Note: "hej"}) {
		t.Fatalf("heard %+v", heard)
	}
}

// auditOnly is an event no typed listener hears.
type auditOnly struct{ Note string }

func (auditOnly) EventName() string { return "audit.only" }

func TestARemovedListenerIsGivenUp(t *testing.T) {
	database, dispatcher, relay, ctx := setup(t)
	events.On(dispatcher, func(context.Context, taskDone) error { return nil }, events.Name("kept"))
	if err := relay.Store(ctx, taskDone{ID: 1}); err != nil {
		t.Fatal(err)
	}
	// A delivery for a listener the code no longer has.
	database.DB.Exec(`INSERT INTO event_deliveries (event_id, listener) VALUES (1, 'removed')`)

	if err := relay.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	d := deliveries(t, database)
	if d["removed"].delivered == "" || d["removed"].lastError == "" || d["kept"].delivered == "" || d["kept"].lastError != "" {
		t.Fatalf("deliveries: %+v", d)
	}
}

func TestPublishAllInATransactionDeliversAfterTheCommit(t *testing.T) {
	database, dispatcher, relay, ctx := setup(t)
	var heard atomic.Int32
	events.On(dispatcher, func(context.Context, taskDone) error { heard.Add(1); return nil }, events.Name("log"))

	err := db.Transaction(ctx, func(ctx context.Context) error {
		if err := relay.PublishAll(ctx, taskDone{ID: 1}); err != nil {
			return err
		}
		if heard.Load() != 0 {
			t.Error("nothing may be delivered before the commit")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if heard.Load() != 1 {
		t.Fatalf("after the commit the listener ran %d times, want 1", heard.Load())
	}

	// After a rollback nothing is stored or delivered.
	db.Transaction(ctx, func(ctx context.Context) error {
		relay.PublishAll(ctx, taskDone{ID: 2})
		return errFlaky
	})
	if heard.Load() != 1 || len(eventRows(t, database)) != 1 {
		t.Fatalf("a rolled back PublishAll: heard %d, %d events", heard.Load(), len(eventRows(t, database)))
	}
}

func TestRetryFreesStaleClaimsAndStopsAfterMaxAttempts(t *testing.T) {
	database, dispatcher, relay, ctx := setup(t)
	var heard atomic.Int32
	events.On(dispatcher, func(context.Context, taskDone) error { heard.Add(1); return nil }, events.Name("log"))
	if err := relay.Store(ctx, taskDone{ID: 1}); err != nil {
		t.Fatal(err)
	}
	if err := relay.Store(ctx, taskDone{ID: 2}); err != nil {
		t.Fatal(err)
	}
	// Event 1 was claimed by a process that died; event 2 used all its attempts.
	database.DB.Exec(`UPDATE domain_events SET claimed_at = '2000-01-01 00:00:00' WHERE id = 1`)
	database.DB.Exec(`UPDATE domain_events SET attempts = ? WHERE id = 2`, outbox.MaxAttempts)

	if err := relay.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	if heard.Load() != 0 {
		t.Fatal("Flush must not take a claimed event or one without attempts left")
	}
	if err := relay.Retry(ctx); err != nil {
		t.Fatal(err)
	}
	if heard.Load() != 1 {
		t.Fatalf("Retry must free the stale claim and deliver once, heard %d", heard.Load())
	}
	if rows := eventRows(t, database); rows[1].published != "" {
		t.Fatalf("an event without attempts left must stay: %+v", rows[1])
	}
}

func TestSchema(t *testing.T) {
	for _, driver := range []string{"sqlite", "Postgres"} {
		if s, err := outbox.Schema(driver); err != nil || s == "" {
			t.Fatalf("Schema(%q): %v", driver, err)
		}
	}
	if _, err := outbox.Schema("mysql"); err == nil {
		t.Fatal("an unknown driver must give an error")
	}
}

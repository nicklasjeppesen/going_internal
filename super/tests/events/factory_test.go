package events_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/nicklasjeppesen/going_internal/super/customrouter"
	"github.com/nicklasjeppesen/going_internal/super/events"
)

// Tests for listener factories: On with a func whose parameters come from the
// dispatcher's Resolver (WithResolver) and that returns the listener.

// journal and mailer are a factory's dependencies.
type journal interface{ Write(line string) }
type mailer interface{ Send(to string) }

type memoryJournal struct{ lines []string }

func (j *memoryJournal) Write(line string) { j.lines = append(j.lines, line) }

type memoryMailer struct{ sent []string }

func (m *memoryMailer) Send(to string) { m.sent = append(m.sent, to) }

// LogRegistered is a factory with one dependency.
func LogRegistered(j journal) func(context.Context, userRegistered) error {
	return func(_ context.Context, e userRegistered) error {
		j.Write("registered " + e.Email)
		return nil
	}
}

// WelcomeRegistered is a factory with two dependencies.
func WelcomeRegistered(j journal, m mailer) func(context.Context, userRegistered) error {
	return func(_ context.Context, e userRegistered) error {
		m.Send(e.Email)
		j.Write("welcomed " + e.Email)
		return nil
	}
}

// wiring is a container with a journal and a mailer, and a dispatcher that
// resolves through it.
func wiring(t *testing.T) (*events.Dispatcher, *memoryJournal, *memoryMailer) {
	t.Helper()
	j, m := &memoryJournal{}, &memoryMailer{}
	container := customrouter.NewContainer().
		Register(func() journal { return j }).
		Register(func() mailer { return m })
	dispatcher := events.New(events.WithResolver(container))
	t.Cleanup(dispatcher.Shutdown)
	return dispatcher, j, m
}

func TestOnTakesAFactoryAndResolvesItsDependencies(t *testing.T) {
	dispatcher, j, m := wiring(t)

	events.On(dispatcher, LogRegistered)
	events.On(dispatcher, WelcomeRegistered, events.Async())

	want := []events.ListenerInfo{
		{Name: "events_test.LogRegistered"},
		{Name: "events_test.WelcomeRegistered", Async: true},
	}
	if got := dispatcher.Listeners(userRegistered{}); !reflect.DeepEqual(got, want) {
		t.Fatalf("Listeners: got %+v, want %+v", got, want)
	}
	if typ, ok := dispatcher.EventType("user.registered"); !ok || typ != reflect.TypeFor[userRegistered]() {
		t.Fatalf("EventType: %v %v", typ, ok)
	}

	if err := dispatcher.Dispatch(context.Background(), userRegistered{Email: "a@b.dk"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(j.lines, []string{"registered a@b.dk", "welcomed a@b.dk"}) || !reflect.DeepEqual(m.sent, []string{"a@b.dk"}) {
		t.Fatalf("journal %q, mailer %q", j.lines, m.sent)
	}
}

func TestAFactoryIsCalledOnceAtRegistration(t *testing.T) {
	dispatcher, _, _ := wiring(t)
	calls := 0
	countingFactory = func(journal) func(context.Context, userRegistered) error {
		calls++
		return func(context.Context, userRegistered) error { return nil }
	}
	events.On(dispatcher, countingFactory, events.Name("counting"))
	for range 3 {
		dispatcher.Dispatch(context.Background(), userRegistered{})
	}
	if calls != 1 {
		t.Fatalf("the factory ran %d times", calls)
	}
}

// countingFactory is set by TestAFactoryIsCalledOnceAtRegistration.
var countingFactory func(journal) func(context.Context, userRegistered) error

func TestAFactoryCalledByHandIsAHandler(t *testing.T) {
	dispatcher := events.New() // no Resolver needed
	defer dispatcher.Shutdown()
	j := &memoryJournal{}

	events.On(dispatcher, LogRegistered(j))
	if got := dispatcher.Listeners(userRegistered{}); len(got) != 1 || got[0].Name != "events_test.LogRegistered" {
		t.Fatalf("Listeners: %+v", got)
	}
	dispatcher.Dispatch(context.Background(), userRegistered{Email: "x"})
	if len(j.lines) != 1 {
		t.Fatalf("journal: %q", j.lines)
	}
}

func TestNameOverridesAFactorysName(t *testing.T) {
	dispatcher, _, _ := wiring(t)
	events.On(dispatcher, LogRegistered, events.Name("user.log"))
	if got := dispatcher.Listeners(userRegistered{}); len(got) != 1 || got[0].Name != "user.log" {
		t.Fatalf("Listeners: %+v", got)
	}
}

// panicMessage is the message call panics with, or "" without a panic.
func panicMessage(call func()) (message string) {
	defer func() {
		if r := recover(); r != nil {
			message = r.(error).Error()
		}
	}()
	call()
	return ""
}

func TestOnRejectsWhatIsNeitherAHandlerNorAFactory(t *testing.T) {
	dispatcher, _, _ := wiring(t)
	bare := events.New()
	defer bare.Shutdown()

	cases := []struct {
		name     string
		call     func()
		contains string
	}{
		{"not a func", func() { events.On(dispatcher, 42) }, "expects a listener"},
		{"nil", func() { events.On(dispatcher, nil) }, "expects a listener"},
		{"no context", func() { events.On(dispatcher, func(userRegistered) error { return nil }) }, "expects a listener func(context.Context, E) error"},
		{"no error", func() { events.On(dispatcher, func(context.Context, userRegistered) {}) }, "expects a listener"},
		{"not an event", func() { events.On(dispatcher, func(context.Context, string) error { return nil }) }, "expects a listener"},
		{"an interface event", func() { events.On(dispatcher, func(context.Context, events.Event) error { return nil }) }, "expects a listener"},
		{"factory returns the wrong shape", func() { events.On(dispatcher, func(journal) func(userRegistered) error { return nil }) }, "returns func(events_test.userRegistered) error"},
		{"factory parameter not resolvable", func() {
			events.On(dispatcher, func(*strings.Builder) func(context.Context, userRegistered) error { return nil })
		}, "needs *strings.Builder (param 0), which the Resolver can't resolve"},
		{"factory without a Resolver", func() { events.On(bare, LogRegistered) }, "has no Resolver"},
		{"factory returns nil", func() {
			events.On(dispatcher, func(journal) func(context.Context, userRegistered) error { return nil })
		}, "returned a nil listener"},
	}
	for _, c := range cases {
		if got := panicMessage(c.call); !strings.Contains(got, c.contains) {
			t.Errorf("%s: panic %q, want it to contain %q", c.name, got, c.contains)
		}
	}
}

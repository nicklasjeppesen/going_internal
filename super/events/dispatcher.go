package events

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"sync"
	"sync/atomic"
)

// Defaults for a dispatcher that is created without options.
const (
	defaultWorkers = 4
	defaultBuffer  = 256
)

// Dispatcher routes domain events to the listeners that registered for them.
//
// The zero value is not usable, create a dispatcher with New.
//
// A dispatcher owns a pool of workers, so events published with Publish are
// handled outside the calling goroutine. Call Shutdown when the application
// stops, so the events that are still in the queue are handled before the
// process exits.
type Dispatcher struct {

	// queue holds the events waiting to be handled by the worker pool.
	queue chan delivery

	// stop is closed by Shutdown to tell the workers to drain the queue
	// and return.
	stop     chan struct{}
	stopOnce sync.Once

	// workers tracks the running worker goroutines.
	workers sync.WaitGroup

	// listeners holds the registered handlers per event name, guarded by mu.
	mu        sync.RWMutex
	listeners map[string][]registration

	// types is the Go type of each event name, learned from the typed
	// listeners (On, ListenTo), guarded by mu.
	types map[string]reflect.Type

	// closed flips to true as soon as Shutdown starts, so no new event is
	// accepted after that point.
	closed atomic.Bool

	// workerCount and bufferSize are applied by the options in New.
	workerCount int
	bufferSize  int

	// onError receives every error an asynchronous listener returns.
	onError func(error)

	// resolver resolves the parameters of listener factories (WithResolver).
	resolver Resolver

	// failures collects the errors of the asynchronous listeners, guarded
	// by failureMu.
	failureMu sync.Mutex
	failures  []error
}

// delivery is a single event travelling through the worker pool.
type delivery struct {
	ctx   context.Context
	event Event
}

// Option customises a dispatcher while it is created.
type Option func(*Dispatcher)

// Resolver resolves a value by its type, like a DI container
// (customrouter.Container fulfils it). It is false when nothing is
// registered for t.
type Resolver interface {
	Resolve(t reflect.Type) (reflect.Value, bool)
}

// WithResolver lets On take listener factories: funcs whose parameters are
// resolved through r and that return the listener (see On).
//
//	dispatcher := events.New(events.WithResolver(container))
func WithResolver(r Resolver) Option {
	return func(dispatcher *Dispatcher) {
		dispatcher.resolver = r
	}
}

// WithWorkers sets how many listeners may handle events at the same time.
// Values below one keep the default.
func WithWorkers(count int) Option {
	return func(dispatcher *Dispatcher) {
		if count > 0 {
			dispatcher.workerCount = count
		}
	}
}

// WithBuffer sets how many events may wait in the queue before Publish
// blocks. Values below one keep the default.
func WithBuffer(size int) Option {
	return func(dispatcher *Dispatcher) {
		if size > 0 {
			dispatcher.bufferSize = size
		}
	}
}

// WithErrorHandler registers a callback that receives every error an
// asynchronous listener returns. Without a callback the errors are only kept
// for Failures, which means they would go unnoticed.
//
// The handler runs on a worker goroutine and may be called from several of
// them at once, so it has to be safe for concurrent use.
func WithErrorHandler(handler func(error)) Option {
	return func(dispatcher *Dispatcher) {
		dispatcher.onError = handler
	}
}

// New creates a dispatcher and starts its workers.
//
// Register the listeners right after, the events can be queued before that,
// they are handled as soon as a worker picks them up.
func New(options ...Option) *Dispatcher {
	dispatcher := &Dispatcher{
		listeners:   make(map[string][]registration),
		types:       make(map[string]reflect.Type),
		stop:        make(chan struct{}),
		workerCount: defaultWorkers,
		bufferSize:  defaultBuffer,
	}

	for _, option := range options {
		option(dispatcher)
	}

	dispatcher.queue = make(chan delivery, dispatcher.bufferSize)

	for worker := 0; worker < dispatcher.workerCount; worker++ {
		dispatcher.workers.Add(1)
		go dispatcher.work()
	}

	return dispatcher
}

// Listen registers a handler for the events published under name.
//
// Registering the same name twice is not an error, the handlers are simply
// called in the order they were registered.
//
// A handler registered with Listen follows the call that delivers the event:
// it runs on the worker pool for Publish and inline for Dispatch. Use On to
// let the listener decide that itself.
func (dispatcher *Dispatcher) Listen(name string, handler Handler) {
	dispatcher.register(name, registration{handler: handler, mode: modeFollowCaller})
}

// register stores a handler together with the way it wants to be called.
//
// It panics when a named listener has an empty name or would share its name
// with another listener of the same event (see Name).
func (dispatcher *Dispatcher) register(name string, entry registration) {
	dispatcher.mu.Lock()
	defer dispatcher.mu.Unlock()

	if entry.named {
		if err := dispatcher.checkName(name, entry.name); err != nil {
			panic(err)
		}
	}
	if entry.eventType != nil {
		if known, ok := dispatcher.types[name]; ok && known != entry.eventType {
			panic(fmt.Errorf("events: %q is already the name of %v, not of %v", name, known, entry.eventType))
		}
		dispatcher.types[name] = entry.eventType
	}

	dispatcher.listeners[name] = append(dispatcher.listeners[name], entry)
}

// EventType is the Go type of the events published under name, learned when a
// typed listener (On, ListenTo) was registered for it. False when no typed
// listener has been registered for the name (Listen and ListenAll don't know
// the type).
//
// An outbox uses it to decode a stored event back into its own type.
func (dispatcher *Dispatcher) EventType(name string) (reflect.Type, bool) {
	dispatcher.mu.RLock()
	defer dispatcher.mu.RUnlock()

	typ, ok := dispatcher.types[name]
	return typ, ok
}

// checkName reports an error when listenerName can't be used for a listener
// of eventName: it is empty, or another listener of the same event already
// has it. A wildcard listener is part of every event, so its name is checked
// against all of them. The caller holds mu.
func (dispatcher *Dispatcher) checkName(eventName, listenerName string) error {
	if listenerName == "" {
		return fmt.Errorf("events: a listener for %q was given an empty name", eventName)
	}

	var lists = []string{eventName, Wildcard}
	if eventName == Wildcard {
		lists = slices.Sorted(maps.Keys(dispatcher.listeners))
	}

	for _, list := range lists {
		for _, existing := range dispatcher.listeners[list] {
			if existing.name == listenerName {
				return fmt.Errorf("events: the listener name %q is already used for %q", listenerName, list)
			}
		}
	}

	return nil
}

// ListenAll registers a handler that is called for every published event,
// no matter its name. Like Listen it follows the caller; pass Async to always
// run it on the worker pool, and Name to give it a stable name (a named
// wildcard listener is listed and delivered for every event).
func (dispatcher *Dispatcher) ListenAll(handler Handler, options ...ListenOption) {
	var entry = registration{handler: handler, mode: modeFollowCaller}
	for _, option := range options {
		option(&entry)
	}

	dispatcher.register(Wildcard, entry)
}

// ListenTo registers a handler that receives the event as its own type.
//
// The routing key is taken from the zero value of T, so T has to be a non
// pointer type that implements Event.
//
//	events.ListenTo(dispatcher, func(ctx context.Context, event domain.UserRegistered) error {
//		log.Println("welcome", event.Email)
//		return nil
//	})
//
// Pass Name to give it a stable name (see Deliver). Without Async it follows
// the caller like Listen; with Async it always runs on the worker pool for
// Publish, like On.
//
// It is a function and not a method on the Dispatcher, because Go does not
// allow a method to declare its own type parameters.
func ListenTo[T Event](dispatcher *Dispatcher, handler func(ctx context.Context, event T) error, options ...ListenOption) {
	var prototype T
	var name = prototype.EventName()

	var entry = registration{
		mode:      modeFollowCaller,
		eventType: concreteType[T](),
		handler: func(ctx context.Context, event Event) error {
			typed, ok := event.(T)
			if !ok {
				return fmt.Errorf("listener for %q expected %T, received %T", name, prototype, event)
			}

			return handler(ctx, typed)
		},
	}

	entry.nameAfter(handler)
	for _, option := range options {
		option(&entry)
	}

	dispatcher.register(name, entry)
}

// Forget drops every handler registered for name, which mostly helps when a
// test has to get rid of a listener again.
func (dispatcher *Dispatcher) Forget(name string) {
	dispatcher.mu.Lock()
	defer dispatcher.mu.Unlock()

	delete(dispatcher.listeners, name)
	delete(dispatcher.types, name)
}

// HasListeners reports whether anything is listening for name.
func (dispatcher *Dispatcher) HasListeners(name string) bool {
	dispatcher.mu.RLock()
	defer dispatcher.mu.RUnlock()

	return len(dispatcher.listeners[name]) > 0
}

// Names returns the event names that currently have a handler.
func (dispatcher *Dispatcher) Names() []string {
	dispatcher.mu.RLock()
	defer dispatcher.mu.RUnlock()

	names := make([]string, 0, len(dispatcher.listeners))
	for name := range dispatcher.listeners {
		names = append(names, name)
	}

	slices.Sort(names)

	return names
}

// Dispatch calls the handlers of the event in the calling goroutine and
// returns their errors joined together.
//
// A handler that fails never stops the rest, and a handler that panics is
// recovered and reported as an error, so one broken listener cannot take the
// caller down. Use this when the caller has to know that the event was fully
// handled, for example when it is part of a transaction.
//
// Errors are returned instead of being reported to the error handler,
// because the caller sees them right here.
func (dispatcher *Dispatcher) Dispatch(ctx context.Context, event Event) error {
	var errs []error

	for _, handler := range dispatcher.handlersFor(event, everyMode) {
		if err := ctx.Err(); err != nil {
			errs = append(errs, fmt.Errorf("events: listener for %q was not called: %w", event.EventName(), err))
			break
		}

		if err := invoke(ctx, event, handler); err != nil {
			errs = append(errs, fmt.Errorf("events: listener for %q failed: %w", event.EventName(), err))
		}
	}

	return errors.Join(errs...)
}

// Publish delivers the event to its listeners.
//
// The listeners registered with On without Async run first, in the calling
// goroutine, and their errors are returned. Every other listener is handed
// to the worker pool, so a request is never blocked by the slow side effects
// an event triggers.
//
// When a sync listener fails, the event is not handed to the worker pool at
// all. The caller is told the event failed, so no async side effect may run
// for it. Every sync listener is still called, one failing sync listener never
// stops the next one.
//
// Handing the event to the pool blocks only while the queue is full, and gives
// up when the context is done or the dispatcher shuts down. Errors from the
// queued listeners are not returned, they go to the error handler and to
// Failures instead.
func (dispatcher *Dispatcher) Publish(ctx context.Context, event Event) error {
	if dispatcher.closed.Load() {
		return ErrShutdown
	}

	if err := dispatcher.runInline(ctx, event); err != nil {
		return err
	}

	return dispatcher.enqueue(ctx, event)
}

// runInline calls the synchronous listeners of the event in the calling
// goroutine and returns their errors joined together.
func (dispatcher *Dispatcher) runInline(ctx context.Context, event Event) error {
	var errs []error

	for _, handler := range dispatcher.handlersFor(event, onlySync) {
		if err := ctx.Err(); err != nil {
			errs = append(errs, fmt.Errorf("events: listener for %q was not called: %w", event.EventName(), err))
			break
		}

		if err := invoke(ctx, event, handler); err != nil {
			errs = append(errs, fmt.Errorf("events: listener for %q failed: %w", event.EventName(), err))
		}
	}

	return errors.Join(errs...)
}

// enqueue hands the event to the worker pool, which calls the listeners that
// are not synchronous.
func (dispatcher *Dispatcher) enqueue(ctx context.Context, event Event) error {
	select {
	case dispatcher.queue <- delivery{ctx: ctx, event: event}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-dispatcher.stop:
		return ErrShutdown
	}
}

// PublishDetached is Publish for an event that outlives its caller.
//
// The context keeps its values but is never cancelled, which is what a
// listener needs when the event is published from a http request: the request
// context is done the moment the response is sent, so a listener that used it
// directly would find the context cancelled before it was even called.
//
// A detached listener is never cancelled either, so a listener that blocks
// forever keeps Shutdown waiting. Long running work should set a timeout of
// its own, for example with context.WithTimeout inside the listener.
func (dispatcher *Dispatcher) PublishDetached(ctx context.Context, event Event) error {
	return dispatcher.Publish(context.WithoutCancel(ctx), event)
}

// Listeners returns the named listeners of the event, in the order they were
// registered: the event's own first, then the named listeners for every event
// (Wildcard). Unnamed listeners are left out.
//
// An app that delivers to one listener at a time uses it to know which
// deliveries an event needs, for example one outbox row per listener.
func (dispatcher *Dispatcher) Listeners(event Event) []ListenerInfo {
	dispatcher.mu.RLock()
	defer dispatcher.mu.RUnlock()

	var name = event.EventName()
	var infos []ListenerInfo

	for _, list := range [][]registration{dispatcher.listeners[name], dispatcher.listeners[Wildcard]} {
		for _, entry := range list {
			if entry.named {
				infos = append(infos, ListenerInfo{Name: entry.name, Async: entry.mode == modeAsync})
			}
		}
	}

	return infos
}

// Deliver runs only the listener called name for the event, inline in the
// calling goroutine whatever its mode, and returns its error. A panic is
// recovered and returned as an error.
//
// It is the building block for retrying one side effect at a time: when an
// app stores a delivery per listener, a failing mail is retried without
// running the activity log again. Publish, PublishDetached and Dispatch are
// not affected and still run every listener.
//
// The error wraps ErrNoListener when the event has no listener with that
// name, and is ErrShutdown after Shutdown.
func (dispatcher *Dispatcher) Deliver(ctx context.Context, event Event, name string) error {
	if dispatcher.closed.Load() {
		return ErrShutdown
	}

	handler, ok := dispatcher.namedHandler(event, name)
	if !ok {
		return fmt.Errorf("events: %q has no listener called %q: %w", event.EventName(), name, ErrNoListener)
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("events: listener %q for %q was not called: %w", name, event.EventName(), err)
	}

	if err := invoke(ctx, event, handler); err != nil {
		return fmt.Errorf("events: listener %q for %q failed: %w", name, event.EventName(), err)
	}

	return nil
}

// namedHandler finds the listener called name among the event's listeners and
// the ones for every event.
func (dispatcher *Dispatcher) namedHandler(event Event, name string) (Handler, bool) {
	dispatcher.mu.RLock()
	defer dispatcher.mu.RUnlock()

	for _, list := range [][]registration{dispatcher.listeners[event.EventName()], dispatcher.listeners[Wildcard]} {
		for _, entry := range list {
			if entry.named && entry.name == name {
				return entry.handler, true
			}
		}
	}

	return nil, false
}

// Failures returns the errors collected from the asynchronous handlers.
func (dispatcher *Dispatcher) Failures() []error {
	dispatcher.failureMu.Lock()
	defer dispatcher.failureMu.Unlock()

	return slices.Clone(dispatcher.failures)
}

// ClearFailures drops the collected errors and returns how many were removed.
func (dispatcher *Dispatcher) ClearFailures() int {
	dispatcher.failureMu.Lock()
	defer dispatcher.failureMu.Unlock()

	var dropped = len(dispatcher.failures)
	dispatcher.failures = nil

	return dropped
}

// Shutdown stops accepting new events and waits until the queued events have
// been handled. It is safe to call more than once.
//
// The queued events are drained, not thrown away, but a handler that blocks
// forever keeps Shutdown waiting, so a handler should always respect its
// context.
func (dispatcher *Dispatcher) Shutdown() {
	dispatcher.stopOnce.Do(func() {
		dispatcher.closed.Store(true)
		close(dispatcher.stop)
	})

	dispatcher.workers.Wait()
	dispatcher.discard()
}

// work handles queued events until Shutdown is called and the queue is empty.
func (dispatcher *Dispatcher) work() {
	defer dispatcher.workers.Done()

	for {
		select {
		case item := <-dispatcher.queue:
			dispatcher.handle(item)
		case <-dispatcher.stop:
			dispatcher.drain()
			return
		}
	}
}

// drain handles the events that are already queued, so nothing accepted by
// Publish is lost when the application shuts down.
func (dispatcher *Dispatcher) drain() {
	for {
		select {
		case item := <-dispatcher.queue:
			dispatcher.handle(item)
		default:
			return
		}
	}
}

// discard frees the queue after the workers are gone, so a Publish that was
// already waiting for room is not stuck on a full queue.
func (dispatcher *Dispatcher) discard() {
	for {
		select {
		case <-dispatcher.queue:
		default:
			return
		}
	}
}

// handle calls every handler of the event and reports what failed.
func (dispatcher *Dispatcher) handle(item delivery) {
	for _, handler := range dispatcher.handlersFor(item.event, skipSync) {
		if err := item.ctx.Err(); err != nil {
			dispatcher.fail(fmt.Errorf("events: listener for %q was not called: %w", item.event.EventName(), err))
			return
		}

		if err := invoke(item.ctx, item.event, handler); err != nil {
			dispatcher.fail(fmt.Errorf("events: listener for %q failed: %w", item.event.EventName(), err))
		}
	}
}

// handlersFor returns the handlers for the event that the filter accepts, the
// ones listening to every event included.
func (dispatcher *Dispatcher) handlersFor(event Event, accept func(mode) bool) []Handler {
	dispatcher.mu.RLock()
	defer dispatcher.mu.RUnlock()

	var name = event.EventName()
	var handlers = make([]Handler, 0, len(dispatcher.listeners[name])+len(dispatcher.listeners[Wildcard]))

	for _, list := range [][]registration{dispatcher.listeners[name], dispatcher.listeners[Wildcard]} {
		for _, entry := range list {
			if accept(entry.mode) {
				handlers = append(handlers, entry.handler)
			}
		}
	}

	return handlers
}

// fail keeps the error and hands it to the error handler.
func (dispatcher *Dispatcher) fail(err error) {
	dispatcher.failureMu.Lock()
	dispatcher.failures = append(dispatcher.failures, err)
	dispatcher.failureMu.Unlock()

	if dispatcher.onError != nil {
		dispatcher.onError(err)
	}
}

// invoke calls the handler and turns a panic into an error, so a listener can
// never bring down the request or the worker handling it.
func invoke(ctx context.Context, event Event, handler Handler) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("panicked: %v", recovered)
		}
	}()

	return handler(ctx, event)
}

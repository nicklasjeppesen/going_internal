package events

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"strings"
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

	// name is the stable name given with Name or derived from the handler's
	// function (FuncName), and named is true when it has one (also with Name(""),
	// which register rejects).
	name  string
	named bool

	// eventType is the event's Go type for a typed listener (On, ListenTo),
	// nil for Listen and ListenAll. The dispatcher remembers it per event name
	// (see EventType).
	eventType reflect.Type
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

// Name gives the listener a stable name, so it can be listed with Listeners
// and run on its own with Deliver.
//
// It is optional for On and ListenTo: without it a listener is named after
// its function (see FuncName), so events.On(d, listeners.LogTodoCreated(todos))
// is called "listeners.LogTodoCreated". Name overrides that name. Give Name to
// an inline func literal (otherwise it stays unnamed), to tell apart two
// listeners made by the same factory for one event, and to pin the name.
//
// An app that stores deliveries (for example one outbox row per event and
// listener) keeps the name in its database, so the name is part of its data.
// A derived name changes when the function is renamed or moved to another
// package; deliveries stored under the old name then find no listener.
// Write the name out by hand when that matters.
//
//	events.On(dispatcher, listeners.SendWelcomeMail, events.Async(), events.Name("user.welcome-mail"))
//
// The name must not be empty, and must be unique per event: registering a
// second listener with the same name for the same event panics, since that is
// a wiring mistake that should stop the app at startup. A named listener for
// every event (Wildcard) shares its name with all events.
func Name(name string) ListenOption {
	return func(entry *registration) {
		entry.name, entry.named = name, true
	}
}

// On registers a listener that receives the event as its own type, and that
// decides itself how it is called.
//
// listener is either
//
//   - a handler, func(ctx context.Context, event E) error, where E is a
//     concrete type that implements Event, or
//
//   - a factory: a func that takes the listener's dependencies and returns
//     such a handler, like a controller's Loader. On calls it once, right
//     away, with its parameters resolved through the dispatcher's Resolver
//     (see WithResolver).
//
//     events.On(dispatcher, listeners.ConfirmOrder)                   // a handler
//     events.On(dispatcher, listeners.LogTodoCompleted)               // a factory: func(todo.Repository) func(ctx, TodoCompleted) error
//     events.On(dispatcher, listeners.LogTodoCompleted(todos))        // the same factory, called by hand
//     events.On(dispatcher, listeners.NotifyCustomer, events.Async()) // worker pool
//
// Without options the listener is synchronous: it runs before Publish returns,
// and its error is returned by Publish. Pass Async to run it on the worker
// pool instead. The one publishing the event never has to know which is which.
// The listener is named after the function passed (the factory, for a
// factory), see Name.
//
// Dispatch still runs every listener inline, whatever its mode.
//
// On panics at registration when listener is neither, when a factory's
// parameter can't be resolved, or when a factory is passed to a dispatcher
// without WithResolver.
func On(dispatcher *Dispatcher, listener any, options ...ListenOption) {
	handler, eventType := dispatcher.handlerOf(listener)
	name := reflect.Zero(eventType).Interface().(Event).EventName()

	var entry = registration{
		mode:      modeSync,
		eventType: eventType,
		handler: func(ctx context.Context, event Event) error {
			if event == nil || reflect.TypeOf(event) != eventType {
				return fmt.Errorf("listener for %q expected %v, received %T", name, eventType, event)
			}
			out := handler.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(event)})
			err, _ := out[0].Interface().(error)
			return err
		},
	}

	entry.nameAfter(listener)
	for _, option := range options {
		option(&entry)
	}

	dispatcher.register(name, entry)
}

// Types On checks listeners against.
var (
	contextType = reflect.TypeFor[context.Context]()
	errorType   = reflect.TypeFor[error]()
	eventIface  = reflect.TypeFor[Event]()
)

// handlerOf is the handler listener is or (for a factory) returns, and its
// event type. It panics when listener is neither (see On).
func (dispatcher *Dispatcher) handlerOf(listener any) (reflect.Value, reflect.Type) {
	value := reflect.ValueOf(listener)
	if !value.IsValid() || value.Kind() != reflect.Func || value.IsNil() {
		panic(fmt.Errorf("events: On expects a listener func(context.Context, Event) error or a factory that returns one, got %T", listener))
	}
	fnType := value.Type()
	if eventType, ok := handlerEvent(fnType); ok {
		return value, eventType
	}
	if fnType.NumOut() != 1 || fnType.IsVariadic() {
		panic(fmt.Errorf("events: On expects a listener func(context.Context, E) error (E a concrete Event) or a factory that returns one, got %s", fnType))
	}
	eventType, ok := handlerEvent(fnType.Out(0))
	if !ok {
		panic(fmt.Errorf("events: On expects a listener func(context.Context, E) error (E a concrete Event) or a factory that returns one; %s returns %s", fnType, fnType.Out(0)))
	}

	// A factory: resolve its parameters and call it once.
	if dispatcher.resolver == nil {
		panic(fmt.Errorf("events: %s is a listener factory, but the dispatcher has no Resolver; create it with events.New(events.WithResolver(container)) or call the factory yourself", fnType))
	}
	args := make([]reflect.Value, fnType.NumIn())
	for i := range args {
		param := fnType.In(i)
		dep, ok := dispatcher.resolver.Resolve(param)
		if !ok {
			panic(fmt.Errorf("events: the listener factory %s needs %s (param %d), which the Resolver can't resolve", FuncName(listener), param, i))
		}
		if !dep.IsValid() {
			dep = reflect.Zero(param)
		}
		args[i] = dep
	}
	handler := value.Call(args)[0]
	if handler.IsNil() {
		panic(fmt.Errorf("events: the listener factory %s returned a nil listener", FuncName(listener)))
	}
	return handler, eventType
}

// handlerEvent is E when fnType is func(context.Context, E) error with E a
// concrete type that implements Event.
func handlerEvent(fnType reflect.Type) (reflect.Type, bool) {
	if fnType.Kind() != reflect.Func || fnType.NumIn() != 2 || fnType.NumOut() != 1 || fnType.IsVariadic() {
		return nil, false
	}
	if fnType.In(0) != contextType || fnType.Out(0) != errorType {
		return nil, false
	}
	eventType := fnType.In(1)
	if eventType.Kind() == reflect.Interface || !eventType.Implements(eventIface) {
		return nil, false
	}
	return eventType, true
}

// nameAfter names the listener after handler, the function the caller passed
// (see FuncName). Name, applied after it, overrides it. An inline func literal
// written in the function that registers it (the caller of On or ListenTo) is
// left unnamed, as before: every literal there would get the same name.
//
// nameAfter must be called straight from On or ListenTo, so it can find their
// caller.
func (entry *registration) nameAfter(handler any) {
	name, closure := funcName(handler)
	if name == "" {
		return
	}
	if closure {
		if pc, _, _, ok := runtime.Caller(2); ok {
			if caller := runtime.FuncForPC(pc); caller != nil {
				if callerName, _ := shortFuncName(caller.Name()); callerName == name {
					return // an inline literal
				}
			}
		}
	}
	entry.name, entry.named = name, true
}

// FuncName is the name On and ListenTo give a listener registered without
// Name: the Go name of the function, short and stable.
//
//   - the package path is dropped up to its last "/":
//     kontoret/internal/app/events/listeners.LogTodoCreated → listeners.LogTodoCreated
//   - closure suffixes (.func1, .func2.1, …) are dropped, so the closure a
//     factory returns is named after the factory:
//     listeners.LogTodoCreated.func1 → listeners.LogTodoCreated
//   - the method value suffix -fm is dropped:
//     listeners.todoListeners.logCompleted-fm → listeners.todoListeners.logCompleted
//
// An inline func literal would get the name of the function it is written in,
// so On and ListenTo leave such a literal unnamed; give it Name when it should
// be listed and delivered. FuncName is "" for nil.
func FuncName(handler any) string {
	name, _ := funcName(handler)
	return name
}

// funcName is FuncName, and whether handler is a closure (a suffix was dropped).
func funcName(handler any) (string, bool) {
	value := reflect.ValueOf(handler)
	if !value.IsValid() || value.Kind() != reflect.Func || value.IsNil() {
		return "", false
	}
	fn := runtime.FuncForPC(value.Pointer())
	if fn == nil {
		return "", false
	}
	return shortFuncName(fn.Name())
}

// shortFuncName shortens a runtime function name (see FuncName), and says
// whether a closure suffix was dropped.
func shortFuncName(name string) (string, bool) {
	name = strings.TrimSuffix(name, "-fm")

	// Drop the package path; type arguments ([…]) may contain "/" themselves.
	head := name
	if i := strings.IndexByte(head, '['); i >= 0 {
		head = head[:i]
	}
	if i := strings.LastIndexByte(head, '/'); i >= 0 {
		name = name[i+1:]
	}

	// Drop closure suffixes: .func1, and the .1, .2 … of nested closures.
	closure := false
	for {
		i := strings.LastIndexByte(name, '.')
		if i < 0 || !isClosureSuffix(name[i+1:]) {
			return name, closure
		}
		name, closure = name[:i], true
	}
}

// isClosureSuffix is true for "func1", "func12", "1" and "12".
func isClosureSuffix(segment string) bool {
	segment = strings.TrimPrefix(segment, "func")
	if segment == "" {
		return false
	}
	for _, r := range segment {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// concreteType is T's type, or nil when T is an interface (an event can't be
// decoded into one).
func concreteType[T Event]() reflect.Type {
	typ := reflect.TypeFor[T]()
	if typ.Kind() == reflect.Interface {
		return nil
	}
	return typ
}

// ListenerInfo describes a named listener of an event.
type ListenerInfo struct {
	// Name is the name given with Name, or derived from the function.
	Name string

	// Async is true when the listener was registered with Async.
	Async bool
}

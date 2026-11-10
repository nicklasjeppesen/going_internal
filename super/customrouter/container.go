package customrouter

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
)

// ---- Dependency injection container ----

// binding is one entry in the container: a factory, called lazily on first
// resolve, and the value it returned, cached from then on (singleton).
type binding struct {
	factory reflect.Value

	// mu guards done and instance, and is held while the factory runs, so it
	// runs only once. Bindings are built in dependency order and Resolve
	// refuses cycles before building, so two of these locks are never waited
	// for the other way round.
	mu       sync.Mutex
	done     bool
	instance reflect.Value
}

// Container is a minimal DI container that maps a type (normally an
// interface) to a factory function that produces it. It resolves the
// parameters of a controller's Loader(...) method, the parameters of other
// factories, and (through events.WithResolver) those of event listener
// factories.
//
// It is safe for concurrent use: controllers call Loader per request, so
// Resolve runs from many goroutines at once.
type Container struct {
	mu       sync.Mutex // guards the map, not the bindings in it
	bindings map[reflect.Type]*binding
}

// NewContainer creates an empty DI container.
func NewContainer() *Container {
	return &Container{bindings: map[reflect.Type]*binding{}}
}

// Register binds a type to a factory function. The type is the factory's
// return type. The factory may take parameters: each is resolved from the
// container by its type, like a Loader's. The dependency is built lazily — the
// first time it's resolved — and then cached (singleton). Registering the same
// type again replaces its binding.
//
// Example:
//
//	container.Register(func() *service.Store { return service.Default() })
//	container.Register(func(store *service.Store) todo.Repository {
//		return infrastructure.NewTodoRepository(store)
//	})
func (c *Container) Register(factory interface{}) *Container {
	factoryType := reflect.TypeOf(factory)
	if factoryType == nil || factoryType.Kind() != reflect.Func {
		panic("customrouter: Register expects a func, e.g. func() helper.ILogger { ... }")
	}
	if factoryType.NumOut() != 1 || factoryType.IsVariadic() {
		panic(fmt.Sprintf("customrouter: the factory passed to Register must return exactly one value and not be variadic, e.g. func(store *Store) helper.ILogger; got %s", factoryType))
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bindings[factoryType.Out(0)] = &binding{factory: reflect.ValueOf(factory)}
	return c
}

// Resolve returns the value bound to t, building it (and what its factory
// needs) the first time. It is false when nothing is registered for t.
//
// It panics when a factory needs a type that isn't registered, or when the
// factories need each other in a cycle; both are wiring mistakes, found the
// first time the binding is resolved (normally at startup).
func (c *Container) Resolve(t reflect.Type) (reflect.Value, bool) {
	b := c.lookup(t)
	if b == nil {
		return reflect.Value{}, false
	}
	if instance, done := b.built(); done {
		return instance, true
	}
	if err := c.check(t, nil); err != nil {
		panic(err)
	}
	return c.build(t, b), true
}

// Resolve returns the value of type T from the container. It panics when
// nothing is registered for T (see Container.Resolve for the other panics).
//
//	todos := customrouter.Resolve[todo.Repository](container)
func Resolve[T any](c *Container) T {
	t := reflect.TypeFor[T]()
	value, ok := c.Resolve(t)
	if !ok {
		panic(fmt.Sprintf("customrouter: nothing is registered for %s", t))
	}
	out, _ := value.Interface().(T) // a nil interface value stays T's zero value
	return out
}

// resolve is Resolve for the Loader code.
func (c *Container) resolve(t reflect.Type) (reflect.Value, bool) { return c.Resolve(t) }

// lookup is the binding of t, or nil.
func (c *Container) lookup(t reflect.Type) *binding {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.bindings[t]
}

// built is the binding's value and true once its factory has run.
func (b *binding) built() (reflect.Value, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.instance, b.done
}

// check walks the factory parameters of t without calling anything, and
// reports a parameter type that isn't registered or a cycle. chain is the
// path of types that led to t.
func (c *Container) check(t reflect.Type, chain []reflect.Type) error {
	for i, seen := range chain {
		if seen == t {
			return fmt.Errorf("customrouter: dependency cycle: %s", chainString(append(append([]reflect.Type{}, chain[i:]...), t)))
		}
	}
	b := c.lookup(t)
	if _, done := b.built(); done {
		return nil
	}

	chain = append(chain[:len(chain):len(chain)], t)
	factoryType := b.factory.Type()
	for i := 0; i < factoryType.NumIn(); i++ {
		param := factoryType.In(i)
		if c.lookup(param) == nil {
			return fmt.Errorf("customrouter: the factory for %s needs %s (param %d), which is not registered (chain: %s)",
				t, param, i, chainString(append(chain[:len(chain):len(chain)], param)))
		}
		if err := c.check(param, chain); err != nil {
			return err
		}
	}
	return nil
}

// build calls b's factory (bound to t) with its resolved parameters, once.
func (c *Container) build(t reflect.Type, b *binding) reflect.Value {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.done {
		return b.instance
	}

	factoryType := b.factory.Type()
	args := make([]reflect.Value, factoryType.NumIn())
	for i := range args {
		param := factoryType.In(i)
		dep := c.lookup(param)
		if dep == nil {
			panic(fmt.Sprintf("customrouter: the factory for %s needs %s (param %d), which is not registered", t, param, i))
		}
		args[i] = c.build(param, dep)
	}
	b.instance = b.factory.Call(args)[0]
	b.done = true
	return b.instance
}

// chainString is "A → B → A".
func chainString(chain []reflect.Type) string {
	names := make([]string, len(chain))
	for i, t := range chain {
		names[i] = t.String()
	}
	return strings.Join(names, " → ")
}

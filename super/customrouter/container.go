package customrouter

import "reflect"

// ---- Dependency injection container ----

// binding is one entry in the container: a not-yet-called factory, built
// lazily on first resolve and then cached.
type binding struct {
	instance reflect.Value
	factory  reflect.Value
}

// Container is a minimal DI container that maps a type (normally an
// interface) to a factory function that produces it. It's used to resolve
// the parameters of a controller's Loader(...) method.
type Container struct {
	bindings map[reflect.Type]*binding
}

// NewContainer creates an empty DI container.
func NewContainer() *Container {
	return &Container{bindings: map[reflect.Type]*binding{}}
}

// Register binds a type to a zero-argument factory function. The type is
// inferred from the factory's return type. The dependency is built lazily —
// the first time it's actually resolved — and then cached (singleton) for
// subsequent resolves:
//
// Example:
//
//	container.Register(func() helper.ILogger {
//		return new(Logger)
//	})
//	container.Register(func() helper.ILogger { return new(Logger) })
func (c *Container) Register(factory interface{}) *Container {
	factoryType := reflect.TypeOf(factory)
	if factoryType == nil || factoryType.Kind() != reflect.Func {
		panic("customrouter: Register expects a func, e.g. func() helper.ILogger { ... }")
	}
	if factoryType.NumIn() != 0 || factoryType.NumOut() != 1 {
		panic("customrouter: factory passed to Register must take no arguments and return exactly one value, e.g. func() helper.ILogger")
	}
	returnType := factoryType.Out(0)
	c.bindings[returnType] = &binding{factory: reflect.ValueOf(factory)}
	return c
}

func (c *Container) resolve(t reflect.Type) (reflect.Value, bool) {
	b, ok := c.bindings[t]
	if !ok {
		return reflect.Value{}, false
	}
	if b.instance.IsValid() {
		return b.instance, true
	}
	// Lazy factory: build once, then cache the result.
	result := b.factory.Call(nil)[0]
	b.instance = result
	return result, true
}

// Package customrouter implements a custom HTTP router that wraps around net/http.ServeMux.
// It supports route grouping, automatic extraction of path parameters, named routes,
// and middleware chaining at both route and router levels.
package customrouter

import (
	"fmt"
	"log"
	"net/http"
	"reflect"
	"regexp"
	"strings"

	routeHelper "github.com/nicklasjeppesen/going_internal/super/customrouter/routeHelper"
	global "github.com/nicklasjeppesen/going_internal/super/global"
	middlewarestdlib "github.com/nicklasjeppesen/going_internal/super/middleware"
	"github.com/nicklasjeppesen/going_internal/super/request"
)

// Modifier defines a function signature that processes an HTTP response and request.
type Modifier func(w http.ResponseWriter, r *http.Request)

// NewMyRouter creates and initializes a new instance of MyRouter.
func NewMyRouter() *MyRouter {
	return &MyRouter{}
}

// MyRouter manages a collection of routes, global middlewares, and an optional URL prefix.
//
// A MyRouter can either be the "root" router (root == nil), which owns the actual
// Handlers slice that gets registered on the http.ServeMux, or a "group" router
// created via Group(), which shares the root's Handlers slice but carries its own
// prefix and middleware chain.
type MyRouter struct {
	// Handlers is the list of registered routes. Only populated/used on the root router.
	Handlers []Route

	// middlewares holds the middlewares that will be attached to every route
	// registered through this router (or a group derived from it).
	middlewares []middlewarestdlib.Middleware

	// prefix is prepended to every URL path registered through this router.
	prefix string

	// root points to the top-level MyRouter that owns Handlers. nil if this *is* the root.
	root *MyRouter

	// container is the DI container used to resolve controller Loader() dependencies.
	// Only meaningful on the root router.
	container *Container
}

// UseContainer wires a DI container into the router. Any controller passed to
// Get/Post/etc. that defines a Loader(...) method will have its dependencies
// resolved from this container.
func (router *MyRouter) UseContainer(container *Container) *MyRouter {
	router.rootRouter().container = container
	return router
}

// rootRouter returns the top-level router that owns the Handlers slice.
func (router *MyRouter) rootRouter() *MyRouter {
	if router.root != nil {
		return router.root
	}
	return router
}

// Group creates a sub-router scoped under the given path prefix.
//
// Example:
//
//	adminRouter := webrouter.Group("/admin").Middleware(someMiddleware)
//	adminRouter.Get("/dashboard", homeController, "Home").Name("admin.dashboard")
//
// Routes registered on the returned router are still stored on the top-level
// router (so a single RegisterRoutes call picks them all up), but automatically
// inherit the combined prefix and middlewares of every Group() in the chain.
func (router *MyRouter) Group(prefix string) *MyRouter {
	return &MyRouter{
		prefix:      router.prefix + prefix,
		middlewares: append([]middlewarestdlib.Middleware{}, router.middlewares...),
		root:        router.rootRouter(),
	}
}

// Middleware appends one or more middlewares to this router/group. Every route
// registered afterwards through this router (or further groups derived from it)
// will pass through these middlewares. Returns itself to allow chaining, e.g.:
//
//	adminRouter := webrouter.Group("/admin").Middleware(authMiddleware)
func (router *MyRouter) Middleware(middlewares ...middlewarestdlib.Middleware) *MyRouter {
	router.middlewares = append(router.middlewares, middlewares...)
	return router
}

// ---- Original uppercase API (kept for backwards compatibility) ----

func (router *MyRouter) GET(path string, handler interface{}) *Route {
	return router.httpHandler("GET", path, handler)
}

func (router *MyRouter) POST(path string, handler interface{}) *Route {
	return router.httpHandler("POST", path, handler)
}

func (router *MyRouter) PUT(path string, handler interface{}) *Route {
	return router.httpHandler("PUT", path, handler)
}

func (router *MyRouter) DELETE(path string, handler interface{}) *Route {
	return router.httpHandler("DELETE", path, handler)
}

func (router *MyRouter) PATCH(path string, handler interface{}) *Route {
	return router.httpHandler("PATCH", path, handler)
}

func (router *MyRouter) OPTIONS(path string, handler interface{}) *Route {
	return router.httpHandler("OPTIONS", path, handler)
}

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

// ---- Controller lifecycle hooks (Rails-style before/after actions) ----

// BeforeAction describes a single "before" hook a controller wants to run
// ahead of one or more of its actions.
//
//   - Name is purely for readability/debugging.
//   - Only, if non-empty, restricts the hook to the listed action (method)
//     names — anything not listed is skipped.
//   - Except, if non-empty (and Only is empty), runs the hook for every
//     action *except* the listed ones.
//   - If both Only and Except are empty, the hook runs for every action.
//   - Handler returns false to abort the request. It's responsible for
//     writing a response itself in that case (e.g. 401/redirect).
//
// Action names are matched case-insensitively against the method name used
// when registering the route (e.g. Get("/x", c, "Show") -> action "Show").
type BeforeAction struct {
	Name    string
	Only    []string
	Except  []string
	Handler func(w http.ResponseWriter, r *http.Request) bool
}

// AfterAction is the "after" counterpart to BeforeAction. It always runs
// once the main action has completed (unless a BeforeAction aborted the
// request first).
type AfterAction struct {
	Name    string
	Only    []string
	Except  []string
	Handler func(w http.ResponseWriter, r *http.Request)
}

// BaseController is meant to be embedded (by value) in your controllers. It
// gives them AddBeforeAction/AddAfterAction, typically called from Loader(),
// e.g.:
//
//	type HomeController struct {
//		customrouter.BaseController
//	}
//
//	func (c *HomeController) Loader() *HomeController {
//		c.AddBeforeAction(customrouter.BeforeAction{
//			Name:    "authenticate_user",
//			Handler: c.authenticateUser,
//		})
//		return c
//	}
type BaseController struct {
	beforeActions []BeforeAction
	afterActions  []AfterAction
}

// AddBeforeAction registers a before-hook on the controller.
func (b *BaseController) AddBeforeAction(action BeforeAction) {
	b.beforeActions = append(b.beforeActions, action)
}

// AddAfterAction registers an after-hook on the controller.
func (b *BaseController) AddAfterAction(action AfterAction) {
	b.afterActions = append(b.afterActions, action)
}

// BeforeActions returns the hooks registered via AddBeforeAction, in order.
func (b *BaseController) BeforeActions() []BeforeAction { return b.beforeActions }

// AfterActions returns the hooks registered via AddAfterAction, in order.
func (b *BaseController) AfterActions() []AfterAction { return b.afterActions }

// beforeActionProvider / afterActionProvider are satisfied by BaseController
// (via the embedded methods above). The router type-asserts against these
// rather than against BaseController directly, so a controller isn't forced
// to embed exactly that struct if it wants to provide the hooks another way.
type beforeActionProvider interface {
	BeforeActions() []BeforeAction
}

type afterActionProvider interface {
	AfterActions() []AfterAction
}

// actionApplies implements the Only/Except matching rules described on BeforeAction.
func actionApplies(actionName string, only, except []string) bool {
	if len(only) > 0 {
		return containsFold(only, actionName)
	}
	if len(except) > 0 {
		return !containsFold(except, actionName)
	}
	return true
}

func containsFold(list []string, s string) bool {
	for _, item := range list {
		if strings.EqualFold(item, s) {
			return true
		}
	}
	return false
}

// ---- New API: Get/Post/etc. that also accept a controller + method name ----

// Get registers a GET route. handlerOrController is either:
//   - a handler function / bound method: Get("/dashboard", homeController.Home)
//   - a controller instance, paired with a method name string:
//     Get("/dashboard", homeController, "Home")
//
// When a controller + method name is used, and the controller defines a
// Loader(...) method, its dependencies are resolved from the router's
// Container (see UseContainer), and any BeforeAction/AfterAction hooks on the
// resolved controller are wired in automatically.
func (router *MyRouter) Get(path string, handlerOrController interface{}, methodName ...string) *Route {
	return router.dispatch("GET", path, handlerOrController, methodName...)
}

func (router *MyRouter) Post(path string, handlerOrController interface{}, methodName ...string) *Route {
	return router.dispatch("POST", path, handlerOrController, methodName...)
}

func (router *MyRouter) Put(path string, handlerOrController interface{}, methodName ...string) *Route {
	return router.dispatch("PUT", path, handlerOrController, methodName...)
}

func (router *MyRouter) Delete(path string, handlerOrController interface{}, methodName ...string) *Route {
	return router.dispatch("DELETE", path, handlerOrController, methodName...)
}

func (router *MyRouter) Patch(path string, handlerOrController interface{}, methodName ...string) *Route {
	return router.dispatch("PATCH", path, handlerOrController, methodName...)
}

func (router *MyRouter) Options(path string, handlerOrController interface{}, methodName ...string) *Route {
	return router.dispatch("OPTIONS", path, handlerOrController, methodName...)
}

// dispatch builds the right Modifier (plain handler vs. resolved controller
// action with DI + before/after hooks) and registers it as a Route.
func (router *MyRouter) dispatch(httpType, path string, handlerOrController interface{}, methodName ...string) *Route {
	root := router.rootRouter()

	var modifier Modifier
	if len(methodName) > 0 {
		modifier = buildControllerAction(root.container, handlerOrController, methodName[0])
	} else {
		modifier = routeHandler(handlerOrController)
	}

	newRoute := Route{
		index:      len(root.Handlers),
		path:       router.prefix + path,
		httpType:   httpType,
		handler:    modifier,
		middleware: append([]middlewarestdlib.Middleware{}, router.middlewares...),
	}

	root.Handlers = append(root.Handlers, newRoute)
	return &root.Handlers[len(root.Handlers)-1]
}

// buildControllerAction resolves a controller's dependencies (via Loader, if
// defined), looks up the target action method, and wraps everything in a
// Modifier that also runs any matching BeforeAction/AfterAction hooks the
// resolved controller has registered.
//
// Resolution (including the Loader call) happens once, at route-registration
// time — not per request — so it behaves like constructor injection. If you
// need per-request/scoped dependencies (e.g. the current user), fetch them in
// a BeforeAction instead, using the request/context.
func buildControllerAction(container *Container, controller interface{}, methodName string) Modifier {
	resolved := resolveController(container, controller)
	action := resolveControllerMethod(resolved, methodName)

	var befores []BeforeAction
	if provider, ok := resolved.(beforeActionProvider); ok {
		befores = provider.BeforeActions()
	}

	var afters []AfterAction
	if provider, ok := resolved.(afterActionProvider); ok {
		afters = provider.AfterActions()
	}

	return func(w http.ResponseWriter, r *http.Request) {
		for _, before := range befores {
			if !actionApplies(methodName, before.Only, before.Except) {
				continue
			}
			if !before.Handler(w, r) {
				return // aborted — handler is responsible for the response
			}
		}

		var urlParamKeys = extractPathParams(r.Pattern)
		var urlParam []string
		for _, key := range urlParamKeys {
			urlParam = append(urlParam, r.PathValue(key))
		}
		request.CallUnknownFunc(action, urlParam, w, r)

		for _, after := range afters {
			if !actionApplies(methodName, after.Only, after.Except) {
				continue
			}
			after.Handler(w, r)
		}
	}
}

// resolveController checks whether the controller defines a Loader(...) method.
// If it does, each of Loader's parameters is treated as an interface type and
// resolved from the container, then Loader is called.
//
// Loader can either:
//   - return the initialized controller (constructor style) — that return
//     value becomes the "real" controller instance, or
//   - return nothing, and instead mutate the controller in place (e.g. via
//     AddBeforeAction) — in which case the original controller (on which
//     Loader was called) is used as-is.
//
// If there's no Loader method at all, the controller is used as-is.
func resolveController(container *Container, controller interface{}) interface{} {
	value := reflect.ValueOf(controller)
	loader := value.MethodByName("Loader")
	if !loader.IsValid() {
		return controller
	}

	loaderType := loader.Type()
	args := make([]reflect.Value, loaderType.NumIn())
	for i := 0; i < loaderType.NumIn(); i++ {
		paramType := loaderType.In(i)
		if container == nil {
			panic(fmt.Sprintf("customrouter: %T defines Loader(...) with parameters but no Container was configured — call router.UseContainer(...)", controller))
		}
		dep, ok := container.resolve(paramType)
		if !ok {
			panic(fmt.Sprintf("customrouter: no dependency registered for %s (required by %T.Loader, param %d)", paramType, controller, i))
		}
		args[i] = dep
	}

	results := loader.Call(args)
	if len(results) > 0 {
		return results[0].Interface()
	}
	return controller
}

// resolveControllerMethod looks up a method by name on a controller instance using
// reflection. Panics (at route-registration time, i.e. at startup) if the method
// doesn't exist, so misconfigured routes fail fast instead of at request time.
//
// Note: if Home has a pointer receiver (func (c *HomeController) Home(...)),
// you must pass a pointer to the controller (e.g. &HomeController{}), not a value.
func resolveControllerMethod(controller interface{}, methodName string) interface{} {
	value := reflect.ValueOf(controller)
	method := value.MethodByName(methodName)
	if !method.IsValid() {
		panic(fmt.Sprintf("customrouter: method %q not found on controller %T (check spelling and pointer receiver)", methodName, controller))
	}
	return method.Interface()
}

// ---- Raw handler API: bypass param-injection/reflection entirely ----

// HandleGet registers a GET route with a plain http.HandlerFunc, e.g.:
//
//	adminRouter.HandleGet("/dashboard", func(w http.ResponseWriter, r *http.Request) {
//		w.Write([]byte("Hello from admin dashboard"))
//	})
func (router *MyRouter) HandleGet(path string, handler http.HandlerFunc) *Route {
	return router.rawHandler("GET", path, handler)
}

func (router *MyRouter) HandlePost(path string, handler http.HandlerFunc) *Route {
	return router.rawHandler("POST", path, handler)
}

func (router *MyRouter) HandlePut(path string, handler http.HandlerFunc) *Route {
	return router.rawHandler("PUT", path, handler)
}

func (router *MyRouter) HandleDelete(path string, handler http.HandlerFunc) *Route {
	return router.rawHandler("DELETE", path, handler)
}

func (router *MyRouter) HandlePatch(path string, handler http.HandlerFunc) *Route {
	return router.rawHandler("PATCH", path, handler)
}

func (router *MyRouter) HandleOptions(path string, handler http.HandlerFunc) *Route {
	return router.rawHandler("OPTIONS", path, handler)
}

// Route represents a registered HTTP endpoint with its layout, method, handler,
type Route struct {
	// Index: position in the webrouter list
	index int
	// path: Current URL path (already includes any Group()/Addprefix() prefixes)
	path string
	// HTTPType: define if it a GET, POST, PUT, DELETE, PATCH or OPTIONS
	httpType string

	// name of the URL
	name string
	// handler: specific handler/controller for the URL
	handler func(w http.ResponseWriter, r *http.Request)

	// Middleware: list of middlewares that have to return true, to reach the URL
	middleware []middlewarestdlib.Middleware
}

// httpHandler is an internal helper that constructs a Route (with parameter
// injection via routeHandler) and appends it to the root router's Handlers.
func (router *MyRouter) httpHandler(HTTPType string, path string, handler interface{}) *Route {
	root := router.rootRouter()

	newRoute := Route{
		index:      len(root.Handlers),
		path:       router.prefix + path,
		httpType:   HTTPType,
		handler:    routeHandler(handler),
		middleware: append([]middlewarestdlib.Middleware{}, router.middlewares...),
	}

	root.Handlers = append(root.Handlers, newRoute)
	return &root.Handlers[len(root.Handlers)-1]
}

// rawHandler is an internal helper that registers a plain http.HandlerFunc
// with no parameter extraction / reflection dispatch.
func (router *MyRouter) rawHandler(HTTPType string, path string, handler http.HandlerFunc) *Route {
	root := router.rootRouter()

	newRoute := Route{
		index:      len(root.Handlers),
		path:       router.prefix + path,
		httpType:   HTTPType,
		handler:    handler,
		middleware: append([]middlewarestdlib.Middleware{}, router.middlewares...),
	}

	root.Handlers = append(root.Handlers, newRoute)
	return &root.Handlers[len(root.Handlers)-1]
}

// Take an controller function (handler) and wrap it in a net/http.ServeMux request
func routeHandler(handler interface{}) Modifier {
	return func(w http.ResponseWriter, r *http.Request) {
		var urlParamKeys = extractPathParams(r.Pattern)
		var urlParam = []string{}
		for _, key := range urlParamKeys {
			urlParam = append(urlParam, r.PathValue(key))
		}
		request.CallUnknownFunc(handler, urlParam, w, r)
	}
}

// ExtractPathParams extracts parameter keys from a path like "/helloworld/{id}/{world}"
func extractPathParams(pathTemplate string) []string {
	re := regexp.MustCompile(`\{([^\}]+)\}`)
	matches := re.FindAllStringSubmatch(pathTemplate, -1)

	var params []string
	for _, match := range matches {
		if len(match) > 1 {
			params = append(params, match[1])
		}
	}
	return params
}

// Name assigns a unique lookup name to the Route.
// It can be use the generate URL for a db model
func (router *Route) Name(name string) *Route {
	router.name = name
	return router
}

// AddMiddleware adds one or more route-specific middlewares to the Route.
// These are processed sequentially before reaching the route's final handler.
func (router *Route) AddMiddleware(middlewares ...func(http.HandlerFunc) http.HandlerFunc) *Route {
	for _, middleware := range middlewares {
		router.middleware = append(router.middleware, middleware)
	}
	return router
}

// GetURL looks up a dynamic route by its assigned name and replaces its curly-brace
//
// - parameters:
//
//   - name: name of the route, user want.
//   - parameters: route path parameter, that shall be exchanges curly brackes in the url.
//
// - return:
//   - (url, error) return (url, ""), if no errors, else empty string and error message.
func (myrouter *MyRouter) GetURL(name string, parameters ...any) string {
	var routeURL string = global.GetRouteNamedMap()[name]
	if routeURL == "" {
		return ""
	}

	var parameterAsStrings []string = routeHelper.ConvertToStrings(parameters)
	var paraLenght int = len(parameterAsStrings)
	routelenght := routeHelper.CountBracedParams(routeURL)

	if paraLenght != routelenght {
		fmt.Println("Not equal lenght")
		return ""
	}

	if replacedRoute, err := routeHelper.ReplaceBracedParams(routeURL, parameterAsStrings); err != nil {
		fmt.Println("error in replace route")
		return ""
	} else {
		return replacedRoute
	}
}

// Addprefix prepends a prefix string to the router's existing prefix configuration.
// Note: only affects routes registered *after* this call, since prefixes are now
// baked into each route's path at registration time (needed for Group() to work).
func (myRouter *MyRouter) Addprefix(prefix string) *MyRouter {
	myRouter.prefix = prefix + myRouter.prefix
	return myRouter
}

// AddmiddlewareGroup registers a group of global middlewares onto the router.
func (myRouter *MyRouter) AddmiddlewareGroup(middlewares middlewarestdlib.MiddlewareGroup) *MyRouter {
	myRouter.middlewares = append(myRouter.middlewares, middlewares...)
	return myRouter
}

// Addmiddleware appends a single global middleware to the router.
func (myRouter *MyRouter) Addmiddleware(middleware middlewarestdlib.Middleware) *MyRouter {
	myRouter.middlewares = append(myRouter.middlewares, middleware)
	return myRouter
}

// RegisterRoutes registers all defined router Handlers into the provided net/http.ServeMux,
// injecting route-specific middleware chains along the way. Paths are already fully
// resolved (prefixes baked in), so no extra prefix is applied here.
func (router *MyRouter) RegisterRoutes(r *http.ServeMux) {
	for _, route := range router.Handlers {
		var handler = route.handler
		var middlewares = route.middleware

		switch route.httpType {
		case "GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS":
			r.HandleFunc(route.httpType+" "+route.path, chain(handler, middlewares))
		default:
			log.Printf("Unsupported HTTP method: %s\n", route.httpType)
		}

		saveNamedRoutes(route)
	}
}

// chain wraps an http.HandlerFunc with a slice of middlewares, processing them
// in reverse order (right to left / bottom to top).
func chain(handler http.HandlerFunc, middlewares middlewarestdlib.MiddlewareGroup) http.HandlerFunc {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}

	return handler
}

// saveNamedRoutes registers a route's path into a global map if a name is provided.
// It panics if a duplicate route name is encountered.
func saveNamedRoutes(route Route) {
	if route.name == "" {
		return
	}

	_, exists := global.GetRouteNamedMap()[route.name]
	if exists {
		panic("double named values of name: " + route.name)
	} else {
		global.SetRouteNamedMap(route.name, route.path)
	}
}

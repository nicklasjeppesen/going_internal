// Package customrouter implements a custom HTTP router that wraps around net/http.ServeMux.
// It supports route grouping, automatic extraction of path parameters, named routes,
// and middleware chaining at both route and router levels.
package customrouter

import (
	"fmt"
	"log"
	"net/http"
	"regexp"

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

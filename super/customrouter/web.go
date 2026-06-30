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
type MyRouter struct {
	// Handlers is a list of registered routes.
	Handlers []Route

	// middlewares holds the global middlewares applied to all routes in this router.
	middlewares []middlewarestdlib.Middleware

	// prefix is prepended to every URL path registered in this router
	// Example:
	// 	.route.GET("/user", ..)
	// 	prefix = api
	// 	URL becomes api/user
	prefix string
}

// Route represents a registered HTTP endpoint with its layout, method, handler,
type Route struct {
	// Index: position in the webrouter list
	index int
	// path: Current URL path
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

// httpHandler is an internal helper that constructs a Route and appends it to the router's Handlers.
func (router *MyRouter) httpHandler(HTTPType string, path string, handler interface{}) *Route {

	newRoute := Route{
		index:    len(router.Handlers),
		path:     path,
		httpType: HTTPType,
		handler:  routeHandler(handler),
	}

	router.Handlers = append(router.Handlers, newRoute)
	return &router.Handlers[len(router.Handlers)-1]

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
func (myRouter *MyRouter) Addprefix(prefix string) *MyRouter {

	myRouter.prefix = prefix + myRouter.prefix
	return myRouter
}

// AddmiddlewareGroup registers a group of global middlewares onto the router.
func (myRouter *MyRouter) AddmiddlewareGroup(middlewares middlewarestdlib.MiddlewareGroup) *MyRouter {
	for _, middleware := range middlewares {
		myRouter.middlewares = append(myRouter.middlewares, middleware)
	}
	myRouter.middlewares = middlewares
	return myRouter
}

// Addmiddleware appends a single global middleware to the router.
func (myRouter *MyRouter) Addmiddleware(middleware middlewarestdlib.Middleware) *MyRouter {
	myRouter.middlewares = append(myRouter.middlewares, middleware)
	return myRouter
}

// RegisterRoutes registers all defined router Handlers into the provided net/http.ServeMux,
// injecting global and route-specific middleware chains along the way.
func (router *MyRouter) RegisterRoutes(r *http.ServeMux) {
	for _, route := range router.Handlers {

		var handler = route.handler
		var middlewares = route.middleware
		var RouterMiddleware = router.middlewares
		var routerPrefix = router.prefix

		switch route.httpType {
		case "GET":
			r.HandleFunc("GET "+routerPrefix+route.path, chain(chain(handler, middlewares), RouterMiddleware))
		case "POST":
			r.HandleFunc("POST "+routerPrefix+route.path, chain(chain(handler, middlewares), RouterMiddleware))
		case "PUT":
			r.HandleFunc("PUT "+routerPrefix+route.path, chain(chain(handler, middlewares), RouterMiddleware))
		case "DELETE":
			r.HandleFunc("DELETE "+routerPrefix+route.path, chain(chain(handler, middlewares), RouterMiddleware))
		case "PATCH":
			r.HandleFunc("PATCH "+routerPrefix+route.path, chain(chain(handler, middlewares), RouterMiddleware))
		case "OPTIONS":
			r.HandleFunc("OPTIONS "+routerPrefix+route.path, chain(chain(handler, middlewares), RouterMiddleware))

		default:
			log.Printf("Unsupported HTTP method: %s\n", route.httpType)
		}

		saveNamedRoutes(route)
	}
}

func (router *MyRouter) groups(basePath string, Indexes ...*Route) {
	var x = len(Indexes)
	var handlerLength = len(router.Handlers)
	for i := len(router.Handlers) - 1; i >= handlerLength-x; i-- {
		router.Handlers[i].path = basePath + router.Handlers[i].path
	}

}

func (router *MyRouter) groupsWithMiddleware(basePath string, middleware middlewarestdlib.Middleware, Indexes ...*Route) {
	var x = len(Indexes)
	var handlerLength = len(router.Handlers)
	for i := len(router.Handlers) - 1; i >= handlerLength-x; i-- {
		router.Handlers[i].path = basePath + router.Handlers[i].path
		router.Handlers[i].middleware = append(router.Handlers[i].middleware, middleware)
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

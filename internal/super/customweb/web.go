package customweb

import (
	"fmt"
	"log"
	global "myapp/internal/super/global"
	middlewarestdlib "myapp/internal/super/middleware"
	"myapp/internal/super/request"
	routeHelper "myapp/internal/super/route"
	"net/http"
	"regexp"
)

type Modifier func(w http.ResponseWriter, r *http.Request)

// Constructor function to initialize MyRouter
func NewMyRouter() *MyRouter {

	return &MyRouter{}
}

type MyRouter struct {
	// Basis a list of routes
	Handlers []Route

	// Middlewares for alle route in the handler group
	middlewares []middlewarestdlib.Middleware

	// Prefix in front of every URL
	// Ex.route.GET("/login", ..)
	// prefix = api
	// URL becomes api/user
	prefix string
}

// Route: Is responsible for holding a given route
// Index: position in the webrouter list
// path: Current URL path
// HTTPType: define if it a GET, POST, PUT, DELETE, PATCH or OPTIONS
// handler: specific handler/controller for the URL
// Middleware: list of middlewares that have to return true, to reach the URL
type Route struct {
	index      int
	path       string
	httpType   string
	name       string
	handler    func(w http.ResponseWriter, r *http.Request)
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

// ExtractPathParams extracts parameter keys from a path like "/hejverden/{id}/{world}"
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

// Set the name of a routing URL
func (router *Route) Name(name string) *Route {
	router.name = name
	return router
}

// AddMiddleware
//
// Add a middleware to a route, that will be process before reaching the handler
func (router *Route) AddMiddleware(middlewares ...func(http.HandlerFunc) http.HandlerFunc) *Route {
	for _, middleware := range middlewares {
		router.middleware = append(router.middleware, middleware)
	}
	return router
}

// GetURL:
//
// - parameters:
//
//   - name: name of the route, user want.
//     parameters: route path parameter, that shall be exchanges with  the parameter in the URL.
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
	var routelenght int = routeHelper.CountBracedParams(routeURL)

	if paraLenght != routelenght {
		fmt.Println("Not equal lenght")
		return ""
	}

	if replacedRoute, err := routeHelper.ReplaceBracedParams(routeURL, parameterAsStrings); err != nil {
		fmt.Println("error in repalaced rotue")
		return ""
	} else {
		return replacedRoute
	}
}

func (myRouter *MyRouter) Addprefix(prefix string) *MyRouter {

	myRouter.prefix = prefix + myRouter.prefix
	return myRouter
}

func (myRouter *MyRouter) Getmiddlewares() []middlewarestdlib.Middleware {
	return myRouter.middlewares
}

func (myRouter *MyRouter) AddmiddlewareGroup(middlewares middlewarestdlib.MiddlewareGroup) *MyRouter {
	for _, middleware := range middlewares {
		myRouter.middlewares = append(myRouter.middlewares, middleware)
	}
	myRouter.middlewares = middlewares
	return myRouter
}

func (myRouter *MyRouter) Addmiddleware(middleware middlewarestdlib.Middleware) *MyRouter {
	myRouter.middlewares = append(myRouter.middlewares, middleware)
	return myRouter
}

// RegisterRoutes
//
// Responsibile for register all the routes to the route provider
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

func (router *MyRouter) Groups(basePath string, Indexes ...*Route) {
	var x = len(Indexes)
	var handlerLength = len(router.Handlers)
	for i := len(router.Handlers) - 1; i >= handlerLength-x; i-- {
		router.Handlers[i].path = basePath + router.Handlers[i].path
	}

}

func (router *MyRouter) GroupsWithMiddleware(basePath string, middleware middlewarestdlib.Middleware, Indexes ...*Route) {
	var x = len(Indexes)
	var handlerLength = len(router.Handlers)
	for i := len(router.Handlers) - 1; i >= handlerLength-x; i-- {
		router.Handlers[i].path = basePath + router.Handlers[i].path
		router.Handlers[i].middleware = append(router.Handlers[i].middleware, middleware)
	}
}

/*
// Check if this is not needed, used to be replaced with chain
func wrapMiddlewares(controller http.HandlerFunc, middleware middlewarestdlib.MiddlewareGroup) http.HandlerFunc {
	return chain(controller, middleware)
}
*/

// createFunc
//
// Takes the type mod and transform it into a similar type,
// This is because of Gos lack of covariance
/*
func createFunc(mod Modifier) func(w http.ResponseWriter, r *http.Request) {
	return func(a http.ResponseWriter, b *http.Request) {
		mod(a, b)
	}
}*/

// Chain
//
// applies a series of middleware to a handler
func chain(handler http.HandlerFunc, middlewares middlewarestdlib.MiddlewareGroup) http.HandlerFunc {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}

	return handler
}

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

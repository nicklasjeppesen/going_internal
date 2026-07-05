// SocketRouter is responsible for providing a route register functions
// For all websocket routes in the application
package socket

import (
	"fmt"
	"net/http"

	web "github.com/nicklasjeppesen/going_internal/super/customrouter"
)

//-----------------------------------------------------------------
// 							SocketRouter
//-----------------------------------------------------------------
//
// SocketRouter is responsible for providing a route register functions
// For all websocket routes in the application

type Router struct {
	Webrouter *web.MyRouter
	Manager   *Manager

	// container is used to resolve dependencies for hubs that define a
	// Loader(...) method. Set via UseContainer.
	container *web.Container
}

func NewSocketRouter() Router {
	return Router{Webrouter: &web.MyRouter{}, Manager: NewManager()}
}

// UseContainer wires a DI container into the socket router. Any hub passed to
// MapHub that defines a Loader(...) method will have its dependencies
// resolved from this container before it's wired up.
func (socket *Router) UseContainer(container *web.Container) *Router {
	socket.container = container
	return socket
}

func (socket *Router) MapHub(path string, hub IBaseHub, middlewares ...func(http.HandlerFunc) http.HandlerFunc) {

	hub = resolveHub(socket.container, hub)

	hub.SetupDefaultHub()
	hub.RegisterRoutes()
	hub.SetbaseURL(path)

	socket.Manager.AllHubs[hub.GetBaseURl()] = hub

	socket.Webrouter.Get(path, func(w http.ResponseWriter, r *http.Request) {
		socket.Manager.serveWS(w, r, hub)
	}).AddMiddleware(middlewares...)
}

// resolveHub runs the hub through the shared customrouter DI mechanism: if
// the hub defines a Loader(...) method, its parameters are resolved from the
// container (same Loader convention used by HTTP controllers), and whatever
// Loader returns (or the mutated hub itself, if Loader has no return value)
// is used from then on. If the hub has no Loader method, it's returned
// unchanged.
func resolveHub(container *web.Container, hub IBaseHub) IBaseHub {
	resolved := web.ResolveDependencies(container, hub)

	resolvedHub, ok := resolved.(IBaseHub)
	if !ok {
		panic(fmt.Sprintf("socket: %T no longer implements IBaseHub after Loader() ran — Loader must return something implementing IBaseHub, or nothing at all", resolved))
	}
	return resolvedHub
}

// Register websocket routes to routing func
func (socket *Router) RegisterRoutes(r *http.ServeMux) {
	socket.Webrouter.RegisterRoutes(r)
	socket.Manager.GetAppMessages()
}

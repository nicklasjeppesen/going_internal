package socket

import (
	web "myapp/internal/super/customweb"
	"net/http"
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
}

func NewSocketRouter() Router {
	return Router{Webrouter: &web.MyRouter{}, Manager: NewManager()}
}

func (socket *Router) MapHub(path string, hub IBaseHub, middlewares ...func(http.HandlerFunc) http.HandlerFunc) {

	hub.SetupDefaultHub()
	hub.RegisterRoutes()
	hub.SetbaseURL(path)

	socket.Manager.AllHubs[hub.GetBaseURl()] = hub

	socket.Webrouter.GET(path, func(w http.ResponseWriter, r *http.Request) {
		socket.Manager.serveWS(w, r, hub)
	}).AddMiddleware(middlewares...)
}

// Register websocket routes to routing func
func (socket *Router) RegisterRoutes(r *http.ServeMux) {
	socket.Webrouter.RegisterRoutes(r)
	socket.Manager.GetAppMessages()
}

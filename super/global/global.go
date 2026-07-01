// global is a general a place to store global information.
// information that a created when start up, and only read med execution.
package global

// routeNameMap
// key: name of a route
// value: URL of a route
//
// ex.
// name = user.show
// value = /users/show/{id}
var routeNameMap = make(map[string]string)

//GetRouteNamedMap return the entire map of stored routes
func GetRouteNamedMap() map[string]string {
	return routeNameMap
}

// SetRouteNamedMap store a url by its name, for later retriving in the app.
func SetRouteNamedMap(name string, url string) {
	routeNameMap[name] = url
}

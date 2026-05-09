package global

// routeNameMap
// key: name of a route
// value: URL of a route
//
// ex.
// name = user.show
// value = /users/show/{id}
var routeNameMap = make(map[string]string)

func GetRouteNamedMap() map[string]string {
	return routeNameMap
}

func SetRouteNamedMap(name string, url string) {
	routeNameMap[name] = url
}

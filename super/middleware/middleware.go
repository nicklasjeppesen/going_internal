package middleware

import "github.com/nicklasjeppesen/going_internal/super/request"

// Adjust the module path as needed

// Middleware is a function that wraps an http.Handler
//type Middleware func(http.HandlerFunc) http.HandlerFunc
//type MiddlewareGroup []Middleware

type Middleware func(request.Handler) request.Handler
type MiddlewareGroup []Middleware

func Chain(middlewares ...Middleware) MiddlewareGroup {
	return middlewares

}

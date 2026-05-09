package middleware

import (
	// Adjust the module path as needed
	"net/http"
)

// Middleware is a function that wraps an http.Handler
type Middleware func(http.HandlerFunc) http.HandlerFunc

type MiddlewareGroup []Middleware

func Chain(middlewares ...Middleware) MiddlewareGroup {
	return middlewares

}

package middleware

import (
	//cors "github.com/nicklasjeppesen/going_internal/config/cors"
	"net/http"

	"github.com/nicklasjeppesen/going_internal/super/request"
)

// Check if the origin is allowed
func isOriginAllowed(origin string, allowedOrigins []string) bool {
	for _, allowedOrigin := range allowedOrigins {
		if origin == allowedOrigin {
			return true
		}
	}
	return false
}

// CORS middleware to handle multiple origins
func Cors(next request.Handler, allowedOrigins []string) request.Handler {
	return func(req *request.Requestbase) {
		origin := req.R.Header.Get("Origin")

		if origin != "" && isOriginAllowed(origin, allowedOrigins) {
			req.W.Header().Set("Access-Control-Allow-Origin", origin)
			req.W.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		req.W.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		req.W.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-CSRF-Token")

		if req.R.Method == "OPTIONS" {
			req.W.WriteHeader(http.StatusOK)
			return
		}

		next(req)
	}
}

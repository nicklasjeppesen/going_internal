package response

import (
	"net/http"
)

/*
* Struct to handle different kind of response Fails, a controller can return.
*
 */
type Fail struct {
}

// status 400
func (c *Fail) StatusBadRequest(message string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, message, http.StatusBadRequest)
	}
}

// status 401
func (c *Fail) StatusUnauthorized(message string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, message, http.StatusUnauthorized)
	}
}

// status 403
func (c *Fail) StatusForbidden(message string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, message, http.StatusForbidden)
	}
}

// status 404
func (c *Fail) StatusNotFound(message string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, message, http.StatusNotFound)
	}
}

// status 500
func (c *Fail) StatusInternalServerError(message string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, message, http.StatusInternalServerError)
	}
}

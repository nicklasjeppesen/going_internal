package request

import (
	"encoding/json"
	"errors"
	"net/http"

	auth "github.com/nicklasjeppesen/going_internal/super/auth"
	. "github.com/nicklasjeppesen/going_internal/super/result"
	. "github.com/nicklasjeppesen/going_internal/super/validation"
)

type Requestbase struct {
	W http.ResponseWriter // index 0, DO NOT REORDER
	R *http.Request       // index 1, DO NOT REORDER
}

// If input is empty, it print the Request Body
// Else loop through, and print the values.
func (r *Requestbase) PrintJson(values ...any) {
	r.W.Header().Set("Content-Type", "application/json")
	for _, val := range values {
		json.NewEncoder(r.W).Encode(val)
	}
}

func (r *Requestbase) Auth() *auth.Auth {
	return &auth.Auth{W: r.W, R: r.R}
}

// Validate a structs validation
// Return (bool, string)
// bool: symbolize if an error happen.
// string: error message.
func (r *Requestbase) Validate(body interface{}) (bool, string) {
	return Validate(body)
}

func (r *Requestbase) FormValue(key string) string {
	return r.R.FormValue(key)
}

func (r *Requestbase) GetInputs() map[string]interface{} {
	data := make(map[string]interface{})

	for key, values := range r.R.Form {
		for _, value := range values {
			data[key] = value
		}
	}
	return data
}

// Create a request struct, and try to parse the
// request body to the given T type.
type RequestBodybase[T any] struct {
	Requestbase   // index 0, DO NOT REORDER
	Body        T // index 1, DO NOT REORDER
}

func (r *RequestBodybase[T]) GetBody() T {
	return r.Body
}

func (r *RequestBodybase[T]) Validate() *Result[T] {

	if err, errorMessage := r.validate(); err {
		return &Result[T]{Data: r.Body, Error: true, ErrorMessage: errors.New(errorMessage)}
	}

	if err := Customvalidation(r.Body); err != nil {
		return &Result[T]{Data: r.Body, Error: true, ErrorMessage: err}
	}

	return &Result[T]{Data: r.Body, Error: false, ErrorMessage: nil}
}

func (r *RequestBodybase[T]) validate() (bool, string) {
	return Validate(r.Body)
}

/**
 * If input is empty, it print the Request Body
 * Else loop through, and print the values.
 */
func (r *RequestBodybase[T]) PrintJson(values ...any) {

	r.W.Header().Set("Content-Type", "application/json")
	if len(values) == 0 {
		json.NewEncoder(r.W).Encode(r.Body)
	} else {
		for _, val := range values {
			json.NewEncoder(r.W).Encode(val)
		}
	}
}

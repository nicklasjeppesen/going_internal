package request

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	auth "github.com/nicklasjeppesen/going_internal/super/auth"
	. "github.com/nicklasjeppesen/going_internal/super/result"
	. "github.com/nicklasjeppesen/going_internal/super/validation"
)

type Handler func(req *Requestbase)

type Requestbase struct {
	W http.ResponseWriter // index 0, DO NOT REORDER
	R *http.Request       // index 1, DO NOT REORDER
}

func (r *Requestbase) Withcontext(ctx context.Context) *Requestbase {
	r.R = r.R.WithContext(ctx)
	return r
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
func (r *Requestbase) Validate(body interface{}) (bool, map[string][]string) {
	return Validate(body)
}

func (r *Requestbase) FormValue(key string) string {
	return r.R.FormValue(key)
}

// Param is the route parameter key ({key} in the route), or else ?key=… in the
// URL ("" when neither is set) – like Rails' params[:key] for GET values. Form
// and JSON bodies are read with RequestBody[T].
//
//	kind := request.Param("kind")     // /chats/5/files?kind=images → "images"
//	letter := request.Param("letter") // /people/letters/{letter}   → "A"
func (r *Requestbase) Param(key string) string {
	if v := r.R.PathValue(key); v != "" {
		return v
	}
	return r.R.URL.Query().Get(key)
}

// ParamInt is Param as a number: 0 when it is missing or not a number.
//
//	offset := request.ParamInt("offset") // ?offset=20 → 20
func (r *Requestbase) ParamInt(key string) int {
	n, _ := strconv.Atoi(r.Param(key))
	return n
}

// ParamInt64 is Param as an int64 (e.g. ids): 0 when it is missing or not a number.
func (r *Requestbase) ParamInt64(key string) int64 {
	n, _ := strconv.ParseInt(r.Param(key), 10, 64)
	return n
}

func (r *Requestbase) GetInputs() map[string]string {
	data := make(map[string]string)

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

	if ok, errorMessage := Validate(r.Body); !ok {
		return &Result[T]{Data: r.Body, HasError: true, Errors: errorMessage}
	}

	if err := Customvalidation(r.Body); err != nil {
		return &Result[T]{Data: r.Body, HasError: true, Errors: map[string][]string{"error": {err.Error()}}}
	}

	return &Result[T]{Data: r.Body, HasError: false, Errors: map[string][]string{}}
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

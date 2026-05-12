package response

import (
	"encoding/json"
	"fmt"
	"net/http"

	. "github.com/nicklasjeppesen/going_internal/super/util"
)

//
//------------------------------------------------------------------------
// 					Response
//------------------------------------------------------------------------
//
// Response is responsible for generate output to a client
// It receive its input from the controller class, and return it to the user
//

// Struct to handle different kind of response, a controller can return.
type Response struct {
}

// Print a struct to Json
//
// if struct field has hidden:true tag, it will be ignored.
// if the type is is a struct that has a the method: ToJson, this method
// will be called by reflect before casting by Json.Marshal method
func (c *Response) PrintJson(_v any) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		output, _ := ToJSON(_v)
		fmt.Fprintln(w, output)
	}
}

func ToJSON(s any) (string, error) {

	s = HasJsonFunc(s)
	b, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (c *Response) Print(_v any) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, _v)
	}
}

func (c *Response) Redirect(Url string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, Url, 302)
	}
}

func (c *Response) Back(Url string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		referer := r.Referer()

		if referer == "" {
			// fallback hvis browseren ikke sender Referer
			referer = "/"
		}
		http.Redirect(w, r, referer, http.StatusSeeOther)
	}
}

package response

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/nicklasjeppesen/going_internal/super/constants"
	"github.com/nicklasjeppesen/going_internal/super/util"
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
	errorMessage map[string]string
	flashData    map[string]string
}

func NewResponse() *Response {
	response := new(Response)
	response.errorMessage = map[string]string{}
	response.flashData = map[string]string{}
	return response

}

func (response *Response) WithErrors(errors map[string]string) *Response {
	response.errorMessage = errors
	return response
}

func (response *Response) With(data map[string]string) *Response {
	response.flashData = data
	return response
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

		if err := c.setSessionData(r, w); err != nil {
			fmt.Println("Error while saving session", err)
		}

		http.Redirect(w, r, Url, 302)
	}
}

func (c *Response) Back() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		if err := c.setSessionData(r, w); err != nil {
			fmt.Println("Error while saving session", err)
		}

		referer := r.Referer()
		if referer == "" {
			referer = "/"
		}
		http.Redirect(w, r, referer, http.StatusSeeOther)
	}
}

func getSession(r *http.Request) *sessions.Session {
	var key = util.GetEnv(constants.APP_Key, "")
	var store = sessions.NewCookieStore([]byte(key))
	session, _ := store.Get(r, constants.Session_info)
	session.Options.Path = "/"
	return session
}

func (c *Response) setSessionData(request *http.Request, w http.ResponseWriter) error {

	session := getSession(request)
	// Setting error message
	if len(c.errorMessage) > 0 {
		session.Values[constants.Errors] = parseValueToSessions(c.errorMessage)
	}

	// setting flash message
	if len(c.flashData) > 0 {
		session.Values[constants.Flash] = parseValueToSessions(c.flashData)
	}

	// Setting values old
	session.Values[constants.Old] = parseValueToSessions(GetInputs(request))

	// Save the session with the error messages
	err := session.Save(request, w)
	if err != nil {
		return err
	}
	return nil

}

func parseValueToSessions(value any) string {

	encodedValue, _ := json.Marshal(value)
	return string(encodedValue)

}

func GetInputs(r *http.Request) map[string]any {
	data := make(map[string]any)

	for key, values := range r.Form {
		for _, value := range values {
			data[key] = value
		}
	}
	return data
}

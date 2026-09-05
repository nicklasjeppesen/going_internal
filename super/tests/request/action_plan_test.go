package request

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nicklasjeppesen/going_internal/super/customrouter"
	"github.com/nicklasjeppesen/going_internal/super/request"
)

// SubscriptionPayload mirrors what an app controller would bind from a JSON
// body via request.Requestbodybase[*Payload].
type SubscriptionPayload struct {
	Endpoint string `json:"endpoint"`
}

type Result = func(http.ResponseWriter, *http.Request)

// actionA is a controller whose action only takes a plain request.Requestbase
// (no body binding).
type actionA struct{}

func (actionA) Plain(request.Requestbase) Result {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("plain"))
	}
}

// actionB is a controller whose action takes a request.Requestbodybase[*T]
// (body-bound action).
type actionB struct{}

func (actionB) Body(request.RequestBodybase[*SubscriptionPayload]) Result {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("body"))
	}
}

// TestCallPlanDoesNotLeakBetweenActions guards against the arg-plan cache in
// request.getCallPlan being keyed by fnValue.Pointer().
//
// Controller actions are resolved via reflect.Value.MethodByName(...).Interface()
// (see customrouter.resolveControllerMethod). All such method values share the
// same underlying code pointer, so caching by Pointer() leaks one action's
// argument plan to every other action. Serving a Requestbase-only action first,
// then a body-bound Requestbodybase[*T] action, used to panic with:
//
//	reflect: Call using request.Requestbase as type request.Requestbodybase[*T]
func TestCallPlanDoesNotLeakBetweenActions(t *testing.T) {
	router := customrouter.NewMyRouter()
	router.Post("/a", actionA{}, "Plain")
	router.Post("/b", actionB{}, "Body")

	mux := http.NewServeMux()
	router.RegisterRoutes(mux)

	serve := func(path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, io.NopCloser(strings.NewReader(body)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		defer func() {
			if p := recover(); p != nil {
				t.Fatalf("request to %s panicked: %v", path, p)
			}
		}()

		mux.ServeHTTP(rec, req)
		return rec
	}

	// Request 1 populates the plan cache with a Requestbase-only arg plan.
	if rec := serve("/a", ""); rec.Code != http.StatusOK {
		t.Fatalf("/a status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	// Request 2 must build the correct Requestbodybase[*T] arg, not reuse /a's.
	if rec := serve("/b", `{"endpoint":"https://pusher.example"}`); rec.Code != http.StatusOK {
		t.Fatalf("/b status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
}

// TestCallPlanRepeatedCalls_CorrectBodyVerifies the body-bound action actually
// receives the decoded JSON body after the cache has warmed up.
func TestCallPlanRepeatedCalls_CorrectBody(t *testing.T) {
	router := customrouter.NewMyRouter()
	router.Post("/b", actionB{}, "Body")

	mux := http.NewServeMux()
	router.RegisterRoutes(mux)

	body := `{"endpoint":"https://pusher.example"}`
	req := httptest.NewRequest(http.MethodPost, "/b", io.NopCloser(strings.NewReader(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
}

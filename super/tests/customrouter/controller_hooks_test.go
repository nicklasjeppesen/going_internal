package customrouter

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nicklasjeppesen/going_internal/super/customrouter"
	"github.com/nicklasjeppesen/going_internal/super/request"
)

type Result = func(http.ResponseWriter, *http.Request)

// userController sets per-request state (userID) in a BeforeAction and reads
// it in the action — the Rails-style pattern the router must make safe.
type userController struct {
	customrouter.BaseController
	greeting string // set before registration; must survive into every request
	userID   string // per request, set by setUser
	befores  *atomic.Int32
}

func (c *userController) Loader() *userController {
	c.AddBeforeAction(c.setUser).Only("Show")
	return c
}

func (c *userController) setUser(r request.Requestbase) bool {
	c.befores.Add(1)
	c.userID = r.R.Header.Get("X-User")
	return true
}

func (c *userController) Show(request.Requestbase) Result {
	time.Sleep(2 * time.Millisecond) // let concurrent requests overlap
	body := c.greeting + " " + c.userID
	return func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }
}

func (c *userController) Index(request.Requestbase) Result {
	body := "index:" + c.userID
	return func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }
}

func newMux(c *userController) *http.ServeMux {
	router := customrouter.NewMyRouter()
	router.Get("/users", c, "Index")
	router.Get("/users/{id}", c, "Show")
	router.Get("/users/{id}/edit", c, "Show")
	mux := http.NewServeMux()
	router.RegisterRoutes(mux)
	return mux
}

func get(mux *http.ServeMux, path, user string) string {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("X-User", user)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec.Body.String()
}

// Each request must get its own controller instance, so state set by a
// BeforeAction is never seen by (or overwritten by) a concurrent request.
func TestBeforeActionStateIsPerRequest(t *testing.T) {
	c := &userController{greeting: "hej", befores: new(atomic.Int32)}
	mux := newMux(c)

	var wg sync.WaitGroup
	errs := make(chan string, 100)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			user := fmt.Sprintf("user-%d", i)
			if got, want := get(mux, "/users/1", user), "hej "+user; got != want {
				errs <- fmt.Sprintf("got %q, want %q", got, want)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
}

// Loader must not pile up hooks when one controller serves several routes:
// a matching BeforeAction runs exactly once per request.
func TestBeforeActionRunsOncePerRequest(t *testing.T) {
	c := &userController{greeting: "hej", befores: new(atomic.Int32)}
	mux := newMux(c)

	for _, path := range []string{"/users/1", "/users/1/edit"} {
		c.befores.Store(0)
		get(mux, path, "anna")
		if n := c.befores.Load(); n != 1 {
			t.Errorf("%s: before ran %d times, want 1", path, n)
		}
	}

	// Only("Show") — Index must not run it, and sees no leftover state.
	c.befores.Store(0)
	if got := get(mux, "/users", "anna"); got != "index:" {
		t.Errorf("Index got %q, want %q", got, "index:")
	}
	if n := c.befores.Load(); n != 0 {
		t.Errorf("Index: before ran %d times, want 0", n)
	}
}

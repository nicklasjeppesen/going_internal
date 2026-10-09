package customrouter

import (
	"net/http"
	"net/http/httptest"
	"testing"

	middlewarestdlib "github.com/nicklasjeppesen/going_internal/super/middleware"
	"github.com/nicklasjeppesen/going_internal/super/request"
)

// counting returns a middleware that adds one to *n every time it runs.
func counting(n *int) middlewarestdlib.Middleware {
	return func(next request.Handler) request.Handler {
		return func(r *request.Requestbase) {
			*n++
			next(r)
		}
	}
}

func serve(t *testing.T, mux *http.ServeMux, path string) int {
	t.Helper()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec.Code
}

func ok(r *request.Requestbase) { r.W.WriteHeader(http.StatusNoContent) }

// The root router's middlewares run exactly once per request, also for routes
// in groups – whether the groups were made before or after they were added.
func TestRootMiddlewareRunsOnce(t *testing.T) {
	var before, after, group int
	root := NewMyRouter()
	root.Addmiddleware(counting(&before)) // added before the group exists
	g := root.Group("/g").Middleware(counting(&group))
	root.HandleGet("/plain", ok)
	g.HandleGet("/x", ok)
	root.Addmiddleware(counting(&after)) // added after the routes, like routeservice.go

	mux := http.NewServeMux()
	root.RegisterRoutes(mux)

	if code := serve(t, mux, "/plain"); code != http.StatusNoContent {
		t.Fatalf("/plain: status %d", code)
	}
	if before != 1 || after != 1 || group != 0 {
		t.Fatalf("/plain: root middlewares ran %d/%d times (want 1/1), group %d (want 0)", before, after, group)
	}

	before, after = 0, 0
	if code := serve(t, mux, "/g/x"); code != http.StatusNoContent {
		t.Fatalf("/g/x: status %d", code)
	}
	if before != 1 || after != 1 || group != 1 {
		t.Fatalf("/g/x: root ran %d/%d times (want 1/1), group %d (want 1)", before, after, group)
	}
}

// A nested group gets its parent group's middlewares and prefix.
func TestNestedGroupInheritsGroupMiddleware(t *testing.T) {
	var outer, inner int
	root := NewMyRouter()
	g := root.Group("/a").Middleware(counting(&outer))
	g.Group("/b").Middleware(counting(&inner)).HandleGet("/c", ok)

	mux := http.NewServeMux()
	root.RegisterRoutes(mux)
	if code := serve(t, mux, "/a/b/c"); code != http.StatusNoContent || outer != 1 || inner != 1 {
		t.Fatalf("/a/b/c: status %d, outer %d, inner %d (want 204, 1, 1)", code, outer, inner)
	}
}

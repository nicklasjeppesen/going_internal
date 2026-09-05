package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestWithServiceWorkerAllowed_SetsRootScopeHeader guards the Service-Worker-Allowed
// header on asset responses. Without it, a service worker served from /assets/...
// cannot be registered with the root scope "/" (the browser rejects the scope as
// not being "under" the service worker's path).
func TestWithServiceWorkerAllowed_SetsRootScopeHeader(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/assets/js/chat/sw.js", nil)
	withServiceWorkerAllowed(inner).ServeHTTP(rec, req)

	if got := rec.Header().Get("Service-Worker-Allowed"); got != "/" {
		t.Fatalf("Service-Worker-Allowed header = %q, want %q", got, "/")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (inner handler must still be called)", rec.Code)
	}
}

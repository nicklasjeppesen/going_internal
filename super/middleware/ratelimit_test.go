package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	constants "github.com/nicklasjeppesen/going_internal/super/constants"
	"github.com/nicklasjeppesen/going_internal/super/request"
)

func serve(h request.Handler, r *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h(&request.Requestbase{W: rec, R: r})
	return rec
}

func ok(req *request.Requestbase) { req.W.WriteHeader(http.StatusOK) }

func TestRateLimitByIP(t *testing.T) {
	h := RateLimit(3, time.Minute, ByIP)(ok)
	for i := 1; i <= 4; i++ {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "10.0.0.1:1234"
		rec := serve(h, r)
		if want := map[bool]int{true: 200, false: 429}[i <= 3]; rec.Code != want {
			t.Fatalf("request %d: status %d, want %d", i, rec.Code, want)
		}
		if i == 4 && rec.Header().Get("Retry-After") == "" {
			t.Error("missing Retry-After")
		}
	}
	// Another IP has its own counter
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.2:1234"
	if rec := serve(h, r); rec.Code != 200 {
		t.Fatalf("other IP: status %d, want 200", rec.Code)
	}
}

func TestRateLimitWindowResets(t *testing.T) {
	l := &limiter{limit: 1, window: time.Minute, counters: map[string]*windowCounter{}}
	now := time.Now()
	if _, ok := l.allow("k", now); !ok {
		t.Fatal("first request refused")
	}
	if _, ok := l.allow("k", now.Add(time.Second)); ok {
		t.Fatal("second request in window allowed")
	}
	if _, ok := l.allow("k", now.Add(time.Minute)); !ok {
		t.Fatal("request in next window refused")
	}
	// Old windows are swept
	l.allow("other", now.Add(3*time.Minute))
	if _, exists := l.counters["k"]; exists {
		t.Error("finished window not swept")
	}
}

func TestRateLimitByUserAndForm(t *testing.T) {
	byUser := RateLimit(1, time.Minute, ByUser)(ok)
	for _, id := range []string{"7", "8"} {
		r := httptest.NewRequest(http.MethodPost, "/", nil)
		r = r.WithContext(context.WithValue(r.Context(), constants.Auth_id, id))
		if rec := serve(byUser, r); rec.Code != 200 {
			t.Fatalf("user %s first request: %d", id, rec.Code)
		}
	}

	byEmail := RateLimit(1, time.Minute, ByIPAndFormValue("email"))(ok)
	post := func(email string) int {
		r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(url.Values{"email": {email}}.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return serve(byEmail, r).Code
	}
	if post("anna@example.com") != 200 || post("bo@example.com") != 200 {
		t.Fatal("different emails should have separate counters")
	}
	if post("ANNA@example.com ") != 429 {
		t.Fatal("same email (case/space-insensitive) should be limited")
	}
}

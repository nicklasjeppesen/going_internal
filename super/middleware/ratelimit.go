package middleware

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	constants "github.com/nicklasjeppesen/going_internal/super/constants"
	"github.com/nicklasjeppesen/going_internal/super/request"
)

// RateLimit allows at most limit requests per window for each key. Requests
// over the limit get 429 Too Many Requests with a Retry-After header. The key
// function decides what is counted, e.g.:
//
//	// at most 10 login attempts per email and IP per 10 minutes
//	webrouter.Post("/login", loginController.Login).
//		AddMiddleware(middleware.RateLimit(10, 10*time.Minute, middleware.ByIPAndFormValue("email")))
//
//	// at most 120 messages per user per minute (after the auth middleware)
//	webrouter.Post("/message", messageController, "Create").
//		AddMiddleware(JWTMiddleware, middleware.RateLimit(120, time.Minute, middleware.ByUser))
//
// Counters live in memory, per process.
func RateLimit(limit int, window time.Duration, key func(req *request.Requestbase) string) Middleware {
	l := &limiter{limit: limit, window: window, counters: map[string]*windowCounter{}}

	return func(next request.Handler) request.Handler {
		return func(req *request.Requestbase) {
			if retryAfter, ok := l.allow(key(req), time.Now()); !ok {
				req.W.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())+1))
				http.Error(req.W, "Too many requests, try again later", http.StatusTooManyRequests)
				return
			}
			next(req)
		}
	}
}

// ByIP counts per client IP (the connection's address). Behind a reverse
// proxy every request comes from the proxy; use a key function that reads the
// proxy's client header instead.
func ByIP(req *request.Requestbase) string {
	host, _, err := net.SplitHostPort(req.R.RemoteAddr)
	if err != nil {
		return req.R.RemoteAddr
	}
	return host
}

// ByUser counts per logged-in user (set by JWTMiddleware), or per IP for
// anonymous requests. Place it after the auth middleware.
func ByUser(req *request.Requestbase) string {
	if id, ok := req.R.Context().Value(constants.Auth_id).(string); ok && id != "" {
		return "user:" + id
	}
	return "ip:" + ByIP(req)
}

// ByIPAndFormValue counts per IP and form field, e.g. the email on a login
// form: guessing one account's password is slowed down without blocking
// other users behind the same IP.
func ByIPAndFormValue(field string) func(req *request.Requestbase) string {
	return func(req *request.Requestbase) string {
		return ByIP(req) + "|" + strings.ToLower(strings.TrimSpace(req.R.FormValue(field)))
	}
}

type windowCounter struct {
	start time.Time
	count int
}

type limiter struct {
	mu        sync.Mutex
	limit     int
	window    time.Duration
	counters  map[string]*windowCounter
	lastSweep time.Time
}

// allow counts one request for key; if it is over the limit it returns false
// and how long until the window resets.
func (l *limiter) allow(key string, now time.Time) (time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Drop finished windows now and then, so the map doesn't grow forever.
	if now.Sub(l.lastSweep) >= l.window {
		for k, c := range l.counters {
			if now.Sub(c.start) >= l.window {
				delete(l.counters, k)
			}
		}
		l.lastSweep = now
	}

	c := l.counters[key]
	if c == nil || now.Sub(c.start) >= l.window {
		c = &windowCounter{start: now}
		l.counters[key] = c
	}
	c.count++
	if c.count > l.limit {
		return l.window - now.Sub(c.start), false
	}
	return 0, true
}

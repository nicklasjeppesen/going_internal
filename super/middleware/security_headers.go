package middleware

import (
	"net/http"
	"os"

	constants "github.com/nicklasjeppesen/going_internal/super/constants"
)

// SecurityHeaders sets response headers that make the browser refuse common
// attacks. A handler can still set its own value for its response.
//
//   - X-Content-Type-Options: nosniff — never guess a file's type (an uploaded
//     "image" can't be run as a script).
//   - X-Frame-Options / CSP frame-ancestors — the app can't be shown inside
//     another site's frame (clickjacking).
//   - CSP object-src 'none'; base-uri 'self' — no plugins, and injected HTML
//     can't change where relative URLs point. Scripts and styles are not
//     restricted, so existing pages and CDNs keep working.
//   - Referrer-Policy — other sites only see the origin, not full URLs.
//   - Strict-Transport-Security — outside APP_ENV=dev, browsers only use https.
func SecurityHeaders(next http.Handler) http.Handler {
	hsts := os.Getenv(constants.App_env) != constants.Dev

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Content-Security-Policy", "frame-ancestors 'none'; object-src 'none'; base-uri 'self'")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		if hsts && r.TLS != nil {
			h.Set("Strict-Transport-Security", "max-age=31536000")
		}
		next.ServeHTTP(w, r)
	})
}

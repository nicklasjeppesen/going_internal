// Package session provides the one cookie store used for Going's session
// cookie (validation errors, old input and flash messages between requests).
package session

import (
	"crypto/sha256"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/sessions"
	"github.com/nicklasjeppesen/going_internal/super/constants"
	"github.com/nicklasjeppesen/going_internal/super/util"
)

var (
	storeOnce sync.Once
	store     *sessions.CookieStore
)

// Store returns the cookie store, created on first use (after .env is loaded).
//
// The cookie is both signed and encrypted (AES-256), so its content can't be
// read or changed in the browser. Both keys are derived from APP_KEY. It is
// HttpOnly (not readable from JavaScript), SameSite=Lax and Secure (unless
// APP_URL is plain http).
func Store() *sessions.CookieStore {
	storeOnce.Do(func() {
		appKey := util.GetEnv(constants.APP_Key, "")
		hashKey := sha256.Sum256([]byte("going-session-auth:" + appKey))
		blockKey := sha256.Sum256([]byte("going-session-encrypt:" + appKey))

		store = sessions.NewCookieStore(hashKey[:], blockKey[:])
		store.Options.Path = "/"
		store.Options.HttpOnly = true
		store.Options.SameSite = http.SameSiteLaxMode
		// Secure unless the app explicitly runs on plain http (APP_URL=http://...)
		store.Options.Secure = !strings.HasPrefix(util.GetEnv(constants.APP_URL, ""), "http://")
	})
	return store
}

// Get returns the session for the request. A cookie that can't be decoded
// (e.g. written with an older key) gives an empty session.
func Get(r *http.Request) *sessions.Session {
	s, _ := Store().Get(r, constants.Session_info)
	s.Options.Path = "/"
	return s
}

// IsSensitive reports whether a form field must never be stored or shown
// again as "old input" (passwords, tokens, secrets).
func IsSensitive(field string) bool {
	f := strings.ToLower(field)
	for _, word := range []string{"password", "passwd", "secret", "token", "csrf"} {
		if strings.Contains(f, word) {
			return true
		}
	}
	return false
}

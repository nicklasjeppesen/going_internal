package middleware

import (
	// Adjust the module path as needed

	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	constants "github.com/nicklasjeppesen/going_internal/super/constants"
)

// More work is needed here: https://themsaid.com/csrf-protection-go-web-applications
/*
*
* - Maybe this should be updated so CSRF belongs to user in DB.
 */

type Input struct {
	Csrf_token string `json:"csrf_token"`
}

func CsrfMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if r.Method == http.MethodGet {

			cookie, err := r.Cookie(constants.Csrf_token)

			if err == nil && cookie.Valid() == nil {
				ctx := context.WithValue(r.Context(), constants.Csrf_token, cookie.Value)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			csfrToken := generateToken()

			http.SetCookie(w, &http.Cookie{
				Name:     constants.Csrf_token,
				Value:    csfrToken,
				Expires:  time.Now().Add(1 * time.Hour),
				Secure:   true,  // only https
				HttpOnly: false, // Not allowed for http to get
				SameSite: http.SameSiteStrictMode,
			})

			ctx := context.WithValue(r.Context(), constants.Csrf_token, csfrToken)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// For POST/PUT/DELETE requests, Validate
		cookie, err := r.Cookie(constants.Csrf_token)
		if err != nil {
			http.Error(w, "CSRF cookie missing", http.StatusUnauthorized)
			return
		}

		// reading the body
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		r.Body.Close()

		var input Input
		err = json.Unmarshal(bodyBytes, &input)

		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		token := input.Csrf_token
		if token == "" || token != cookie.Value {
			http.Error(w, "Invalid CSRF token", http.StatusUnauthorized)
			return
		}

		// Go's body is a stream, and when it read it is removed, so we have to put it back again. WHAT?
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		// Token valid → Continue
		next.ServeHTTP(w, r)

	})
}

func generateToken() string {
	bytes := make([]byte, 32) // 32 consider minimum for security
	if _, err := rand.Read(bytes); err != nil {
		log.Fatal("Failed to generate token v%", err)
	}
	return base64.URLEncoding.EncodeToString(bytes)
}

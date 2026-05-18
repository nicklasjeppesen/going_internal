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
	"strings"
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

		contentType := r.Header.Get("Content-Type")
		var token string
		if strings.HasPrefix(contentType, "application/json") {
			token = CSRFTokenFromJson(w, r)
		} else {
			token = CSRFTokenFromHttp(w, r, contentType)
		}

		if token == "" || token != cookie.Value {
			http.Error(w, "Invalid CSRF token", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)

	})
}

func CSRFTokenFromHttp(w http.ResponseWriter, r *http.Request, contentType string) string {
	// Standard HTML form request
	if contentType == "multipart/form-data" {
		err := r.ParseMultipartForm(32 << 20) // 32MB max memory
		if err != nil {
			http.Error(w, "Invalid form", http.StatusBadRequest)
			return ""
		}
	}

	return r.FormValue(constants.Csrf_token)
}

func CSRFTokenFromJson(w http.ResponseWriter, r *http.Request) string {

	// reading the body
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return ""
	}
	r.Body.Close()

	var input Input
	err = json.Unmarshal(bodyBytes, &input)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return ""
	}

	token := input.Csrf_token

	// Go's body is a stream, and when it read it is removed, so we have to put it back again. WHAT?
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	// Token valid → Continue
	return token
}

func generateToken() string {
	bytes := make([]byte, 32) // 32 consider minimum for security
	if _, err := rand.Read(bytes); err != nil {
		log.Fatal("Failed to generate token v%", err)
	}
	return base64.URLEncoding.EncodeToString(bytes)
}

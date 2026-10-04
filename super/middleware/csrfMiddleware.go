package middleware

import (
	"crypto/subtle"
	"errors"
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
	"github.com/nicklasjeppesen/going_internal/super/request"
)

// More work is needed here: https://themsaid.com/csrf-protection-go-web-applications
/*
*
* - Maybe this should be updated so CSRF belongs to user in DB.
 */

type Input struct {
	Csrf_token string `json:"csrf_token"`
}

func CsrfMiddleware(next request.Handler) request.Handler {
	return func(req *request.Requestbase) {

		if req.R.Method == http.MethodGet {

			cookie, err := req.R.Cookie(constants.Csrf_token)

			if err == nil && cookie.Valid() == nil {
				ctx := context.WithValue(req.R.Context(), constants.Csrf_token, cookie.Value)

				next(req.Withcontext(ctx))
				return
			}

			csfrToken := generateToken()

			http.SetCookie(req.W, &http.Cookie{
				Name:     constants.Csrf_token,
				Value:    csfrToken,
				Expires:  time.Now().Add(1 * time.Hour),
				Secure:   true,  // only https
				HttpOnly: false, // Not allowed for http to get
				SameSite: http.SameSiteStrictMode,
			})

			ctx := context.WithValue(req.R.Context(), constants.Csrf_token, csfrToken)
			next(req.Withcontext(ctx))
			return
		}

		// For POST/PUT/DELETE requests, Validate
		cookie, err := req.R.Cookie(constants.Csrf_token)
		if err != nil {
			http.Error(req.W, "CSRF cookie missing", http.StatusUnauthorized)
			return
		}

		token, ok := csrfToken(req.W, req.R)
		if !ok {
			return // the body could not be read; the error is already written
		}

		if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(cookie.Value)) != 1 {
			http.Error(req.W, "Invalid CSRF token", http.StatusUnauthorized)
			return
		}

		next(req)

	}
}

// csrfToken reads the submitted token from the JSON or form body. ok is false
// if the body could not be read; the error response (413 for a body over the
// limit, 400 otherwise) has then been written.
func csrfToken(w http.ResponseWriter, r *http.Request) (token string, ok bool) {
	contentType := r.Header.Get("Content-Type")

	if strings.HasPrefix(contentType, "application/json") {
		body, err := io.ReadAll(r.Body)
		r.Body.Close()
		if err != nil {
			writeBodyError(w, err)
			return "", false
		}
		r.Body = io.NopCloser(bytes.NewBuffer(body))

		var input Input
		if err := json.Unmarshal(body, &input); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return "", false
		}
		return input.Csrf_token, true
	}

	var err error
	if strings.HasPrefix(contentType, "multipart/form-data") {
		err = r.ParseMultipartForm(32 << 20) // 32 MB in memory, the rest in temp files
	} else {
		err = r.ParseForm()
	}
	if err != nil {
		writeBodyError(w, err)
		return "", false
	}
	return r.FormValue(constants.Csrf_token), true
}

// writeBodyError answers a request whose body could not be read.
func writeBodyError(w http.ResponseWriter, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
		return
	}
	http.Error(w, "Invalid request body", http.StatusBadRequest)
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

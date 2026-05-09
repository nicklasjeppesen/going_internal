package middleware

import (
	"context"

	constants "github.com/nicklasjeppesen/going_internal/super/constants"

	//. "github.com/nicklasjeppesen/going_internal/internal/super/inertiajs"
	"net/http"

	security "github.com/nicklasjeppesen/going_internal/super/security"
)

func JWTMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(constants.Auth_token)

		// Check if cookie for login token exists
		if err != nil || cookie.Value == "" {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		// Validate login token
		svc := security.NewJWTService()
		token, claim, err := svc.Verify(cookie.Value)

		if err != nil || !token.Valid {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			http.Redirect(w, r, "/", http.StatusMovedPermanently)
			return
		}

		authId := claim.Subject
		ctx := context.WithValue(r.Context(), constants.Auth_id, authId)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

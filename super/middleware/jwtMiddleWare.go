package middleware

import (
	"context"

	constants "github.com/nicklasjeppesen/going_internal/super/constants"
	"github.com/nicklasjeppesen/going_internal/super/request"

	//. "github.com/nicklasjeppesen/going_internal/internal/super/inertiajs"
	"net/http"

	security "github.com/nicklasjeppesen/going_internal/super/security"
)

func JWTMiddleware(next request.Handler) request.Handler {
	return func(req *request.Requestbase) {
		cookie, err := req.R.Cookie(constants.Auth_token)

		referer := req.R.Referer()
		if referer == "" {
			referer = "/"
		}

		// Check if cookie for login token exists
		if err != nil || cookie.Value == "" {

			http.Redirect(req.W, req.R, referer, http.StatusSeeOther)
			return
		}

		// Validate login token
		svc := security.NewJWTService()
		token, claim, err := svc.Verify(cookie.Value)

		if err != nil || !token.Valid {
			http.Error(req.W, "Invalid token", http.StatusUnauthorized)
			http.Redirect(req.W, req.R, referer, http.StatusMovedPermanently)
			return
		}

		authId := claim.Subject
		ctx := context.WithValue(req.R.Context(), constants.Auth_id, authId)
		next(req.Withcontext(ctx))
	}
}

package auth

import (
	"context"
	"crypto/subtle"
	"net/http"

	"errors"
	"github.com/nicklasjeppesen/going_internal/super/constants"
	. "github.com/nicklasjeppesen/going_internal/super/db"
	"github.com/nicklasjeppesen/going_internal/super/db/types"
	. "github.com/nicklasjeppesen/going_internal/super/db/types"
	"github.com/nicklasjeppesen/going_internal/super/security"
	"strconv"
)

type IUser struct {
	ActiveRecord[*IUser]
	Email        string
	Password     string
	SessionToken string
}

func (_user IUser) DB(ctx context.Context) *IUser {
	user := &_user
	user.Table = "users"
	user.Columns = Columns{
		// Column		  "values"
		"email":        &user.Email,
		"password":     &user.Password,
		"sessiontoken": &user.SessionToken,
	}
	user.ParentDB = CreateORM(ctx, user)
	return user
}

// Auth define a authentication stuct for going
type Auth struct {
	Email     string
	Password  string
	TableName string
	Criteria  map[string]any
	W         http.ResponseWriter
	R         *http.Request
	Driver    types.DBCreator
}

func (auth *Auth) UserIdAsString() string {
	userID := auth.R.Context().Value(constants.Auth_id)
	if userID == nil {
		return ""
	}
	return userID.(string)
}

func (auth *Auth) ID() (int64, error) {

	_userID := auth.UserIdAsString()

	if _userID == "" {
		return 0, errors.New("user not found")
	}

	userID, err := strconv.ParseInt(_userID, 10, 64)
	if err != nil {
		return 0, err
	}
	return userID, nil

}

func (auth *Auth) Attempt() bool {
	iUser := new(IUser).DB(auth.R.Context())
	iUser.Where("email", auth.Email)

	if auth.Criteria != nil {
		for column, value := range auth.Criteria {
			iUser.Where(column, value)
		}
	}
	response := iUser.First()
	if !response.Any() {
		security.CheckPasswordAgainstNothing(auth.Password) // same timing as a wrong password
		return false
	}
	if !security.CheckPasswordhash(auth.Password, response.Password) {
		return false
	}

	// The session id is shared by the user's devices; logout clears it, which
	// invalidates every login token issued for it (see SessionValid).
	if response.SessionToken == "" {
		response.SessionToken = security.GenerateToken()
		if err := response.Update(); err != nil {
			return false
		}
	}
	security.LoginUser(response.Id, response.SessionToken, auth.W)
	return true
}

// SessionValid reports whether a login token's session id is still the user's
// current one (not logged out since the token was issued).
func SessionValid(ctx context.Context, userID string, sessionID string) bool {
	if userID == "" || sessionID == "" {
		return false
	}
	user := new(IUser).DB(ctx).Where("id", userID).First()
	return user.Any() && subtle.ConstantTimeCompare([]byte(user.SessionToken), []byte(sessionID)) == 1
}

// Logout ends the user's session: the session id is cleared, so every login
// token issued for it stops working (also a copied or stolen one), and the
// cookies are deleted. The logout route usually has no auth middleware, so
// the user is read from the login cookie when the context has none.
func (auth Auth) Logout() {
	userID := auth.UserIdAsString()
	if userID == "" {
		if cookie, err := auth.R.Cookie(constants.Auth_token); err == nil {
			if _, claims, err := security.NewJWTService().Verify(cookie.Value); err == nil {
				userID = claims.Subject
			}
		}
	}

	user := new(IUser).DB(auth.R.Context()).Where("id", userID).First()
	if user.Any() {
		user.SessionToken = ""
		user.DB(auth.R.Context()).Update()
	}
	security.Logout(auth.W) // delete all sessions
}

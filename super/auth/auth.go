package auth

import (
	"context"
	"net/http"

	"github.com/nicklasjeppesen/going_internal/super/constants"
	. "github.com/nicklasjeppesen/going_internal/super/db"
	"github.com/nicklasjeppesen/going_internal/super/db/types"
	. "github.com/nicklasjeppesen/going_internal/super/db/types"
	"github.com/nicklasjeppesen/going_internal/super/security"
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
		"email":    &user.Email,
		"password": &user.Password,
	}
	user.ParentDB = CreateORM(ctx, user)
	return user
}

// Types
type Auth struct {
	Email     string
	Password  string
	TableName string
	Criteria  map[string]any
	W         http.ResponseWriter
	R         *http.Request
	Driver    types.DBCreator
}

func (auth *Auth) GetUserId() string {
	userId := auth.R.Context().Value(constants.Auth_id)
	if userId == nil {
		return ""
	}
	return userId.(string)
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
	if !response.Any() || !security.CheckPasswordhash(auth.Password, response.Password) {
		return false
	}

	// Store tokens in the database
	token := security.LoginUser(iUser.Id, auth.W)
	iUser.SessionToken = token
	iUser.Update()
	return true
}

func (auth Auth) Logout() {
	user := new(IUser).DB(auth.R.Context()).Where("id", auth.GetUserId()).First()
	if user.Any() {
		user.SessionToken = ""
		user.DB(auth.R.Context()).Update()
	}
	security.Logout(auth.W) // delete all sessions
}

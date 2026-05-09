package auth

import (

	//models "github.com/nicklasjeppesen/going_internal/app/models/db"

	"net/http"

	. "github.com/nicklasjeppesen/going_internal/internal/super/db"
	"github.com/nicklasjeppesen/going_internal/internal/super/db/types"
	. "github.com/nicklasjeppesen/going_internal/internal/super/db/types"
	"github.com/nicklasjeppesen/going_internal/internal/super/security"
)

type IUser struct {
	ActiveRecord[*IUser]
	Email        string
	Password     string
	SessionToken string
}

func (_user IUser) DB() *IUser {
	user := &_user
	user.Table = "users"
	user.Columns = Columns{
		// Column		  "values"
		"email":    &user.Email,
		"password": &user.Password,
	}
	user.ParentDB = CreateORM(user)
	return user
}

// Types
type Auth struct {
	ID string

	EmailName    string
	PasswordName string
	TableName    string
	W            http.ResponseWriter
	Driver       types.DBCreator
}

func (auth *Auth) Attempt(criteria map[string]any) bool {

	iUser := new(IUser).DB()
	for column, value := range criteria {
		iUser.Where(column, value)
	}

	response := iUser.First()
	if !response.Any() || !security.CheckPasswordhash(auth.getPasswordName(criteria), response.Password) {
		return false
	}

	// Store tokens in the database
	token := security.LoginUser(iUser.Id, auth.W)
	iUser.SessionToken = token
	iUser.Update()
	return true
}

func (auth *Auth) getPasswordName(criteria map[string]any) string {
	if auth.PasswordName == "" {
		return criteria["password"].(string)
	} else {
		return criteria[auth.PasswordName].(string)
	}

}

// https://www.youtube.com/watch?v=OmLdoEMcr_Y
/*
func (auth Auth) Login(username string, password string) error {

	user := models.User{}.DB().Where("email", username).First()
	if !user.Any() || !security.CheckPasswordhash(password, user.Password) {
		return errors.New("email and password does not match")
	}

	// Store tokens in the database
	token := security.LoginUser(user.Id, auth.W)
	user.SessionToken = token
	user.Update()

	return nil
}*/

func (auth Auth) Logout() {

	user := new(IUser).DB().Where("id", auth.ID).First()
	if user.Any() {
		user.SessionToken = ""
		user.DB().Update()
	}
	security.Logout(auth.W) // delete all sessions

}

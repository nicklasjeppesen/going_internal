package models

import (
	"context"

	. "github.com/nicklasjeppesen/going_internal/super/db"
	drivers "github.com/nicklasjeppesen/going_internal/super/db/drivers"
	relations "github.com/nicklasjeppesen/going_internal/super/db/relationship"
	. "github.com/nicklasjeppesen/going_internal/super/db/types"
)

type User struct {
	ActiveRecord[*User]

	Name       string
	Age        int64
	Company_id float64
}

func (_user User) DB(ctx context.Context) *User {
	u := &User{}
	u.Table = "users"
	u.Columns = Columns{
		"age":        &u.Age,
		"name":       &u.Name,
		"company_id": &u.Company_id,
	}
	u.ParentDB = CreateORMWithCustomDB(ctx, u, DBCreator{Driver: drivers.CreateSQLite(ctx).Driver, ConnectionString: "./testdb.db"})
	return u
}

// Posts are the user's posts (posts.user_id).
func (u *User) Posts() *relations.HasManyRelation[*Post] {
	return u.HasMany(Post{}.DB(u.GetCtx()))
}

// Post is one of the user's posts (posts.user_id).
func (u *User) Post() *relations.HasOneRelation[*Post] {
	return u.HasOne(Post{}.DB(u.GetCtx()))
}

// Comments are the user's comments (commentable_type = "user").
func (u *User) Comments() *relations.HasManyMorphRelation[*Comment] {
	return u.HasManyMorph(Comment{}.DB(u.GetCtx()))
}

package models

import (
	"context"

	. "github.com/nicklasjeppesen/going_internal/super/db"
	drivers "github.com/nicklasjeppesen/going_internal/super/db/drivers"
	. "github.com/nicklasjeppesen/going_internal/super/db/types"
)

type Post struct {
	ActiveRecord[*Post]

	Title   string
	User_id int64
}

func (_post Post) DB(ctx context.Context) *Post {
	p := &Post{}
	p.Table = "posts"
	p.Columns = Columns{
		"title":   &p.Title,
		"user_id": &p.User_id,
	}
	p.ParentDB = CreateORMWithCustomDB(ctx, p, DBCreator{Driver: drivers.CreateSQLite(ctx).Driver, ConnectionString: "./testdb.db"})
	return p
}

package models

import (
	"context"

	. "github.com/nicklasjeppesen/going_internal/super/db"
	drivers "github.com/nicklasjeppesen/going_internal/super/db/drivers"
	. "github.com/nicklasjeppesen/going_internal/super/db/types"
)

// Comment is a morph child: commentable_type + commentable_id point at its owner.
type Comment struct {
	ActiveRecord[*Comment]

	Body             string
	Commentable_id   int64
	Commentable_type string
}

// DB copies its receiver (as in going_docs' concerns.md), so
// Comment{Body: "..."}.DB(ctx) keeps the body.
func (_comment Comment) DB(ctx context.Context) *Comment {
	c := &_comment
	c.Table = "comments"
	c.Columns = Columns{
		"body":             &c.Body,
		"commentable_id":   &c.Commentable_id,
		"commentable_type": &c.Commentable_type,
	}
	c.ParentDB = CreateORMWithCustomDB(ctx, c, DBCreator{Driver: drivers.CreateSQLite(ctx).Driver, ConnectionString: "./testdb.db"})
	return c
}

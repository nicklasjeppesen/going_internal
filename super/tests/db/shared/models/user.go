package models

import (
	"context"

	. "github.com/nicklasjeppesen/going_internal/super/db"
	drivers "github.com/nicklasjeppesen/going_internal/super/db/drivers"
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

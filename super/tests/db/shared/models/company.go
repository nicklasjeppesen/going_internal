package models

import (
	"context"

	. "github.com/nicklasjeppesen/going_internal/super/db"
	drivers "github.com/nicklasjeppesen/going_internal/super/db/drivers"
	. "github.com/nicklasjeppesen/going_internal/super/db/types"
)

type Company struct {
	ActiveRecord[*Company]
	Name  string
	Users []*User
}

func (c Company) DB(ctx context.Context) *Company {
	company := &Company{}
	company.Table = "companies"
	company.Columns = Columns{
		"name": &company.Name,
	}
	company.ParentDB = CreateORMWithCustomDB(ctx, company, DBCreator{Driver: drivers.CreateSQLite(ctx).Driver, ConnectionString: "./testdb.db"})
	return company
}

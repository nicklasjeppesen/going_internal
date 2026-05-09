package models

import (
	. "github.com/nicklasjeppesen/going_internal/internal/super/db"
	drivers "github.com/nicklasjeppesen/going_internal/internal/super/db/drivers"
	. "github.com/nicklasjeppesen/going_internal/internal/super/db/types"
)

type Company struct {
	ActiveRecord[*Company]
	Name  string
	Users []*User
}

func (c Company) DB() *Company {

	company := &Company{}
	company.Table = "companies"
	company.Columns = Columns{
		// Column		  "values"
		"name": &company.Name,
	}
	company.ParentDB = CreateORMWithCustomDB(company, DBCreator{Driver: drivers.CreateSQLite().Driver, ConnectionString: "./testdb.db"})
	return company
}

// ------------ Relationships ----------------------//
func (company *Company) Relations() IRelationships {

	return company.CreateRelationShip(IRelationships{
		//"users": HasMany(new(User).DB(), &company.Users),
	})
}

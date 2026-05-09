package models

import (
	. "myapp/internal/super/db"
	drivers "myapp/internal/super/db/drivers"

	//. "myapp/internal/super/db/relationship"
	. "myapp/internal/super/db/types"
)

type User struct {
	ActiveRecord[*User]

	Name       string //`json:"name" validate:"required"`
	Age        int64  //`json:"-" validate:"min=0,max=99"`
	Company_id float64

	// Relationship
	Company *Company
}

/**
* very very very important, do not change.
* This is the right method to init the DB,
* It is also used for creating new objects in a GET request, and relationship methods
 */
func (_user User) DB() *User {
	user := &User{}
	user.Table = "users"
	user.Columns = Columns{
		// Column		  "values"
		"age":        &user.Age,
		"name":       &user.Name,
		"company_id": &user.Company_id,
	}

	// Creating DB
	user.ParentDB = CreateORMWithCustomDB(user, DBCreator{Driver: drivers.CreateSQLite().Driver, ConnectionString: "./testdb.db"})
	return user
}

// -------------- Relationships --------------------//

// TODO Implement this
/*
func (user *User) Relations() IRelationships {
	return user.CreateRelationShip(IRelationships{
		"company": BelongsTo_(Company{}.DB(), &user.Company),
	})
}*/

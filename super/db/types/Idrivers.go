package types

import (
	"database/sql"
)

/*
*
* The Concret SQL connection driver,
* This interface specified functions a SQL driver have to implement
* To be used in the system
 */
type IDrivers interface {
	// Non terminnate function
	Open(connectionString string) *sql.DB
	Where_(column string, value []any) IDrivers
	Or_(column string, value any) IDrivers
	WhereIn_(column string, values []any) IDrivers
	SetTable(table string)
	OrderBy_(column string)
	OrderByDesc_(column string)
	Limit_(max int)
	OffSet_(int)

	// Terminate functions
	Get_(_db *sql.DB, columns []string) [][]any
	Save_(_db *sql.DB, columns []string, values []any, returningValues []string) []any
	First_(_db *sql.DB, columns []string) []any // returning columns
	Update_(_db *sql.DB, columns []string, values []any)
	Delete_(_db *sql.DB, id any) error

	// support functions
	CreateMigrationTable() string
}

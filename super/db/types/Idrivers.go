package types

import (
	"database/sql"
)

// The Concret SQL connection driver,
// This interface specified functions a SQL driver have to implement
// To be used in the system
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
	LockForUpdate_()
	SharedLock_()

	// Terminate functions. Database errors are returned (or, for the read
	// functions, logged with an empty result) — never fatal, so one failing
	// query only fails its own request.
	Get_(_db DBTX, columns []string) [][]any
	Save_(_db DBTX, columns []string, values []any, returningValues []string) ([]any, error)
	First_(_db DBTX, columns []string) []any // returning columns
	Update_(_db DBTX, columns []string, values []any) error
	Delete_(_db DBTX, id any) error

	// support functions
	CreateMigrationTable() string
	Migrate(scriptpath string) error
}

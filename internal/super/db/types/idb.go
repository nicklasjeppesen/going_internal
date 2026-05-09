package types

import (
	"database/sql"

	. "github.com/nicklasjeppesen/going_internal/internal/super/collections"
)

/*
*
* Used on the relations structs
 */
type IDBConnection[T IRepository] interface {
	IDB[T]
	GetDriver() IDrivers
	DbConn() *sql.DB
}

/*
*
* Combine all interfaces a for specific Application ActiveRecord (DB ORM)
 */
type IDB[T IRepository] interface {
	IParent[T]
	IModels[T]
	IRepository
}

/*
*
* Interface for Basis SQL queries
 */
type IParent[T IRepository] interface {
	Where(string, ...any) T
	WhereIn(string, []any) T
	WhereMorph(string, ...any) T
	First() T
	With(...string) T
	Get() Collection[T]
	GetWith() []string
	OrderByDesc(column string) T
	OrderBy(column string) T
}

/*
*
* The Application models, required to have a DB function,
 */
type IModels[T IRepository] interface {
	DB() T // return it own type again, very usefull for coping the struct
}

type IRepository interface {
	ISystemFields
	FirstNonGeneric() ISystemFields
	WhereNonGeneric(column string, value ...any) IRepository

	GetNonGeneric() []IRepository
	WhereInNonGeneric(column string, value []any) IRepository
	SaveNonGenerics() (IRepository, error)
}

/*
*
* The SystemFields interface for Basis methods
 */
type ISystemFields interface {
	/*
	* Get the name of the table
	 */
	GetTable() string

	Value(string) (any, error)

	SetValue(string, any) error

	Values() map[string]ValueHolder

	//Return predefined columns, id, created_at, updated_at
	Systemcolumns() []string

	SetRoutes(routes map[string]string)

	// Get the db connection
	DBConnection() DBCreator

	// Set the the Connection
	SetDBConnection(DBCreator)

	// Get a list of values that db shall return
	ReturningValues() []string

	// Get the name of a db table
	GetName() string

	//Add value a list of value to the mode, with the syskeys followed by generalKeys
	AddDBVal(keys []string, syskeys []string, values []any)

	PrimaryKey() any

	PrimaryKeyName() string

	SetPivotsValue(key string, value any) error

	//Get All the columns by name except the system  columnns
	GetKeys() []string

	//Method for copy the model
	CopySelf() IRepository

	//Setter function for copy the model
	SetSelf(func() IRepository)

	SetRelationshipHolder(key string, value any)

	GetRelationshipHolder(key string) []any

	Any() bool
}

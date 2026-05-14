package types

/*
*
* Struct for Creating a getter for a struct property
* This allow the DB to get and set properties of a model struct
 */
type ValueHolder struct {
	Getter    func() any
	Setter    func(any)
	SetterMap func(key string, value any)
}

/*
*
* The map that hold all the getter and setter function for columns values
 */
type DBMapper struct {
	/*
	* key: columnName
	* Value: ValueHolder, include functions for setting and get the struct property
	 */
	ValueHolder map[string]ValueHolder
}

/*
* The Holder for connection string and the driver for connection to the DB
 */
type DBCreator struct {
	Driver           IDrivers
	ConnectionString string
}

type IConcern[A IConcern[A]] interface {
	IDBConnection[A]
}

// Type for concern pattern
type Concern[A IConcern[A]] struct {
	IConcern[A]
}

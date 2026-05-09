package db

import (
	"errors"
	"fmt"
	types "myapp/internal/super/db/types"
	"myapp/internal/super/util"
	"strings"
	"time"
)

// The base struct for all models.
// Responsible for providing basis funtionality to any model struct defined later
type SystemFields struct {
	Id                  int64            `json:"id"`
	Updated_at          time.Time        `json:"updated_at"`
	Created_at          time.Time        `json:"created_at"`
	Table               string           `hidden:"true"`          // table name
	Name                string           `hidden:"true"`          // the name of the model, used for relations
	RelationshipsHolder map[string][]any `hidden:"true" json:"-"` //

	// key: column name
	// value: pointer to model property
	// column mapping between db column name and model property : map[string]any
	Columns   types.Columns            `hidden:"true"`
	Pivots    map[string]any           `json:"pivots"` // pivots map, over values in a many to many relationship
	Routes    map[string]string        `json:"routes"` // routes the the model
	dbCreator types.DBCreator          `json:"-"`
	self      func() types.IRepository `json:"-"`
}

func (dbsys *SystemFields) RelationsToJson() map[string]any {
	return util.MapToJson(dbsys.RelationshipsHolder)
}

func (dbsys *SystemFields) SetRelationshipHolder(key string, value any) {
	if dbsys.RelationshipsHolder == nil {
		dbsys.RelationshipsHolder = map[string][]any{}
	}
	currentVal := dbsys.RelationshipsHolder[key]
	currentVal = append(currentVal, value)
	dbsys.RelationshipsHolder[key] = currentVal
}

func (dbsys *SystemFields) GetRelationshipHolder(key string) []any {
	if val, ok := dbsys.RelationshipsHolder[key]; ok {
		return val
	}
	return []any{}
}

// The setter function for set the function to copy the model
func (dbsys *SystemFields) SetSelf(a func() types.IRepository) {
	dbsys.self = a
}

// Allow the function to return it child type, even it dont know it.
func (dbsys *SystemFields) CopySelf() types.IRepository {
	return dbsys.self()
}

func (dbsys *SystemFields) PrimaryKey() any {
	key, _ := dbsys.Value(dbsys.PrimaryKeyName())
	return key
}

func (dbsys *SystemFields) PrimaryKeyName() string {
	return "id" // in feature, this could change
}

func (dbsys *SystemFields) checkValue(key string) (types.ValueHolder, error) {

	if val, ok := dbsys.DBSetUp().ValueHolder[key]; ok {
		return val, nil
	} else if val, ok := dbsys.SystemMapper().ValueHolder[key]; ok {
		return val, nil
	} else {
		return types.ValueHolder{}, errors.New("key does not exists")
	}
}

// look for a a given value first for system, then for Custom defined value
func (dbsys *SystemFields) Value(key string) (any, error) {
	if val, err := dbsys.checkValue(key); err == nil {
		return val.Getter(), nil
	} else {
		return nil, err
	}
}

// look for a a given value first for system, then for Custom defined value
func (dbsys *SystemFields) Values() map[string]types.ValueHolder {
	systemvalues := dbsys.SystemMapper().ValueHolder
	customValues := dbsys.DBSetUp().ValueHolder

	for key, val := range customValues {
		systemvalues[key] = val
	}
	return systemvalues

}

// Check if a given value
func (dbsys *SystemFields) SetValue(key string, value any) error {
	if val, err := dbsys.checkValue(key); err == nil {
		val.Setter(value)
		return nil
	} else {
		return err
	}
}

// Check if a given value
func (dbsys *SystemFields) SetPivotsValue(key string, value any) error {

	if val, err := dbsys.checkValue("pivots"); err == nil {
		val.SetterMap(key, value)
		return nil
	} else {
		return err
	}
}

// Get current model name, for the conventions
func (dbsys *SystemFields) GetName() string {

	if dbsys.Name != "" {
		return dbsys.Name
	}
	word := dbsys.Table
	lower := strings.ToLower(word)

	// companies -> company
	if strings.HasSuffix(lower, "ies") && len(word) > 3 {
		return word[:len(word)-3] + "y"
	}

	// boxes -> box, classes -> class
	if strings.HasSuffix(lower, "es") {
		return word[:len(word)-2]
	}

	// columns -> column
	if strings.HasSuffix(lower, "s") {
		return word[:len(word)-1]
	}
	return word
}

// Return the the table name
func (dbsys SystemFields) GetTable() string {
	return dbsys.Table
}

// Check if the db query end with a result
func (dbsys *SystemFields) Any() bool {
	return dbsys.Id != 0
}

// Check if the struct is empty
func (dbsys *SystemFields) IsEmpty() bool {
	return dbsys.Id == 0
}

// Function for setting the table in the relation struct, so the struct knows relationee table og reation table
// ex. in the user struct, then the user table is set for them all
func (dbsys *SystemFields) CreateRelationShip(relationshipHolder types.IRelationships) types.IRelationships {
	return relationshipHolder
}

// Get the columns of the model except from the system cols
func (dbsys *SystemFields) GetKeys() []string {
	var keys = []string{}
	for key := range dbsys.DBSetUp().ValueHolder {
		keys = append(keys, key)
	}
	return keys
}

// Data getter and setter for the default columns
func (dbsys *SystemFields) SystemMapper() types.DBMapper {
	var holder = map[string]types.ValueHolder{
		"id":         {Getter: func() any { return dbsys.Id }, Setter: func(val any) { dbsys.Id = val.(int64) }},
		"created_at": {Getter: func() any { return dbsys.Created_at }, Setter: func(val any) { dbsys.Created_at = val.(time.Time) }},
		"updated_at": {Getter: func() any { return dbsys.Updated_at }, Setter: func(val any) { dbsys.Updated_at = val.(time.Time) }},
		"pivots": {Getter: func() any { return dbsys.Pivots }, SetterMap: func(key string, value any) {
			if dbsys.Pivots == nil {
				dbsys.Pivots = map[string]any{}
			}
			dbsys.Pivots[key] = value
		}},
	}
	return types.DBMapper{ValueHolder: holder}
}

// Return default system columns
func (dbsys *SystemFields) Systemcolumns() []string {
	return []string{"id", "created_at", "updated_at"}
}

// Return a list of returning column value from a Save, or update query
func (dbsys *SystemFields) ReturningValues() []string {
	return dbsys.Systemcolumns()
}

// A model can return with possible actions/routes, very usefull for API calls
func (dbsys *SystemFields) SetRoutes(routes map[string]string) {
	dbsys.Routes = routes
}

// Search for a value in systemholder og DBsetup
func (parent *SystemFields) GetValueholderValue(holder types.ISystemFields, key string) any {
	if value, ok := holder.Value(key); ok != nil {
		return value
	}
	return nil
}

// Set value
func (dbsys *SystemFields) SetValueholderValue(key string, value any) error {

	if _, ok := dbsys.SystemMapper().ValueHolder[key]; ok {
		dbsys.SystemMapper().ValueHolder[key].Setter(value)
	} else if _, ok := dbsys.DBSetUp().ValueHolder[key]; ok {
		dbsys.DBSetUp().ValueHolder[key].Setter(value)
	} else {
		return errors.New("key does not exists")
	}
	return nil
}

// Return the default DB connnection based on the env file
// Maybe this should be removed to factory in Driver
func (dbsys *SystemFields) DBConnection() types.DBCreator {
	return dbsys.dbCreator
}

// Return the default DB connnection based on the env file
// Maybe this should be removed to factory in Driver
func (dbsys *SystemFields) SetDBConnection(creator types.DBCreator) {
	dbsys.dbCreator = creator
}

// Add Value from a DB select to the model
func (dbsys *SystemFields) AddDBVal(keys []string, syskeys []string, values []any) {
	if len(values) == 0 {
		return
	}
	for index, key := range keys {
		var value = *(values[index].(*any))
		if value == nil {
			continue
		}
		dbsys.DBSetUp().ValueHolder[key].Setter(value)
	}
	for index, key := range syskeys {
		dbsys.SystemMapper().ValueHolder[key].Setter(*(values[len(keys)+index].(*any)))
	}
}

// Creating the DBMapper for setting and getting columns value
func (dbsys *SystemFields) DBSetUp() types.DBMapper {
	var DBMapper = map[string]types.ValueHolder{}
	for columnName, valuePointer := range dbsys.Columns {
		DBMapper[columnName] = f(valuePointer)
	}
	return types.DBMapper{ValueHolder: DBMapper}
}

// Responsible for creating a Valueholder,
// Valueholder gives a getter and a setter to a struct value
func f(ptr any) types.ValueHolder {
	return types.ValueHolder{
		Getter: func() any {
			switch p := ptr.(type) {
			case *string:
				return *p
			case *int64:
				return *p
			case int64:
				return p
			case *float64:
				return *p
			case float64:
				return p
			default:
				fmt.Println("could not find Getter type " + fmt.Sprintf("%v", p))
				return p
			}
		},
		Setter: func(val any) {
			switch p := ptr.(type) {
			case *string:
				*p = val.(string)
			case *int64:
				*p = val.(int64)
			case *float64:
				*p = val.(float64)
			default:
				fmt.Println("Kunne ikke finde setter type")
				return
			}

		},
	}
}

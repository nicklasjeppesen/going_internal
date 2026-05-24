package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"maps"
	"net/http"
	"reflect"
	"time"

	. "github.com/nicklasjeppesen/going_internal/super/collections"
	drivers "github.com/nicklasjeppesen/going_internal/super/db/drivers"
	. "github.com/nicklasjeppesen/going_internal/super/db/types"
	global "github.com/nicklasjeppesen/going_internal/super/global"
	"github.com/nicklasjeppesen/going_internal/super/route"
	struct_to_map "github.com/nicklasjeppesen/going_internal/super/util"
	validation "github.com/nicklasjeppesen/going_internal/super/validation"
)

// Maybe a good idea?
type ActiveRecord[T IDB[T]] struct {
	*ParentDB[T]
	SystemFields
}

type ParentDB[T IDB[T]] struct {
	creator          DBCreator
	dbChild          *T // any type
	with             []string
	ignorevalidation bool
	route            string
	callback         Responsehandler
	dbconn           *sql.DB
	ctx              context.Context
}

type DataResponse[T any] struct {
	Data    T
	Actions map[string]string
}

type Responsehandler map[string]string

func (parent *ParentDB[T]) GetDriver() IDrivers {
	return parent.creator.Driver
}

func (parent *ActiveRecord[T]) ToJson() map[string]any {

	IgnoreStructs := []string{"Creator"}
	flattenStructs := []string{"ActiveRecord", "ParentDB", "SystemFields"}

	// We need this, because dbChild is private in this scope
	v := reflect.ValueOf(*parent.ParentDB.dbChild)
	t := reflect.ValueOf(parent.SystemFields)

	relations := parent.SystemFields.RelationsToJson()

	child := struct_to_map.Struct_to_map(v, IgnoreStructs, flattenStructs, nil)
	systemFields := struct_to_map.Struct_to_map(t, nil, nil, nil)
	maps.Copy(systemFields, child)
	maps.Copy(systemFields, relations)
	return systemFields

}

func (parent *ParentDB[T]) GetWith() []string {
	return parent.with
}

func (parent *ParentDB[T]) AddRoutes(prefix string, callback ...Responsehandler) *ParentDB[T] {
	parent.route = prefix
	if callback != nil {
		parent.callback = callback[0]
	}
	return parent
}

// search for a key in systemholder og DBsetup
func (parent *ParentDB[T]) addRoutes(data []T) []T {

	var allRoutes = global.GetRouteNamedMap()
	var urls = route.CollectValuesByPrefix(allRoutes, parent.route)

	for _, _data := range data {
		var keyValueURL = make(map[string]string) // map holdning values for for the input parameters.

		for key, val := range _data.Values() {
			keyValueURL[key] = fmt.Sprintf("%v", val.Getter())
		}

		if parent.callback != nil {
			for key, value := range parent.callback {
				keyValueURL[key] = value
			}
		}

		var replaceURL, err = route.ReplaceURLPlaceholders(urls, keyValueURL)
		if err != nil {
			fmt.Println(err.Error())

		}
		_data.SetRoutes(replaceURL)
	}
	return data

}

// Execute a raw select query, and return the result
func (parent *ParentDB[T]) Select(query string) ([]map[string]any, error) {

	var conn = parent.DbConn()
	defer conn.Close()

	rows, err := conn.Query(query)
	if err != nil {
		log.Fatal(err)
		fmt.Println("Error getting rows")
	}

	var columns, _ = rows.Columns()
	records := []map[string]any{}

	for rows.Next() {
		var values = make([]any, len(columns))
		for i := range values {
			values[i] = new(any) // create addressable placeholder
		}

		err := rows.Scan(values[:]...)
		for i, value := range values {
			if value != nil {
				values[i] = *value.(*any)
			}
		}
		if err != nil {
			log.Fatal(err)
			fmt.Println("Error getting rows")
		}
		record := make(map[string]any, len(columns))
		for i, value := range values {
			record[columns[i]] = value
		}

		records = append(records, record)
	}

	// Check for error during iteration
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}

	return records, nil
}

func (parent *ParentDB[T]) With(relation ...string) T {
	parent.with = append(parent.with, relation...)
	return *parent.dbChild
}

func (parent *ParentDB[T]) DbConn() *sql.DB {
	if parent.dbconn != nil {
		return parent.dbconn
	}
	return parent.creator.Driver.Open(parent.creator.ConnectionString)
}

func (parent *ParentDB[T]) SetDbConn(conn *sql.DB) {
	parent.dbconn = conn
}

func (parent *ParentDB[T]) Ignorevalidation() T {
	parent.ignorevalidation = true
	return *parent.dbChild
}

func (parent *ParentDB[T]) Or(column string, value any) T {
	parent.creator.Driver.Or_(column, value)
	return *parent.dbChild
}

/*
* param: value can contain maximum 2 values
* Ex. Where("id", 2) -> Where id = 2
* Ex. Where("id", ">", 2) -> where id > 2
 */
func (parent *ParentDB[T]) Where(column string, value ...any) T {
	parent.creator.Driver.Where_(column, value)
	return *parent.dbChild
}

func (parent *ParentDB[T]) WhereMorph(column string, value ...any) T {
	parent.creator.Driver.Where_(column+"_type", value)
	return *parent.dbChild
}

func (parent *ParentDB[T]) WhereIn(column string, values []any) T {
	parent.creator.Driver.WhereIn_(column, values)
	return *parent.dbChild
}

func (parent *ParentDB[T]) Limit(limit int) T {
	parent.creator.Driver.Limit_(limit)
	return *parent.dbChild
}

func (parent *ParentDB[T]) OffSet(limit int) T {
	parent.creator.Driver.OffSet_(limit)
	return *parent.dbChild
}

func (parent *ParentDB[T]) OrderByDesc(column string) T {
	parent.creator.Driver.OrderByDesc_(column)
	return *parent.dbChild
}

func (parent *ParentDB[T]) OrderBy(column string) T {
	parent.creator.Driver.OrderBy_(column)
	return *parent.dbChild
}

func (parent *ParentDB[T]) SaveNonGenerics() (IRepository, error) {
	return parent.Save()
}

func (parent *ParentDB[T]) Save() (T, error) {

	// Validate input

	if !parent.ignorevalidation {
		if err, errorMessage := validation.Validate(*parent.dbChild); err {
			return *parent.dbChild, errors.New(errorMessage)
		} else if err := validation.Customvalidation(parent.dbChild); err != nil {
			return *parent.dbChild, err
		}
	}

	var _db = parent.DbConn()
	defer _db.Close()

	var object = (*parent.dbChild)
	var keys = object.GetKeys()
	values := make([]any, len(keys))
	for index, v := range keys {
		child := *parent.dbChild
		values[index], _ = child.Value(v)
	}

	returningValues := object.ReturningValues()

	var dbResult = parent.creator.Driver.Save_(parent.DbConn(), keys, values, returningValues)

	for i, value := range returningValues {
		object.SetValue(value, dbResult[i])
	}

	return object, nil
}

func (parent *ParentDB[T]) WhereNonGeneric(column string, value ...any) IRepository {
	return parent.Where(column, value...)
}

func (parent *ParentDB[T]) WhereInNonGeneric(column string, value []any) IRepository {
	return parent.WhereIn(column, value)
}

func (parent *ParentDB[T]) FirstNonGeneric() ISystemFields {
	return parent.First()
}

func (parent ParentDB[T]) GetNonGeneric() []IRepository {
	var generics = parent.Get()
	var nonGenerics = []IRepository{}
	for _, val := range generics {
		nonGenerics = append(nonGenerics, val)
	}
	return nonGenerics
}

/*
*
* return first record, if exists, or empty struct
 */
func (parent *ParentDB[T]) First() T {
	var _db = parent.DbConn()
	defer _db.Close()

	// Run SELECT query
	var keys = (*parent.dbChild).GetKeys()
	child := *parent.dbChild
	var syskeys = child.Systemcolumns()
	var accKeys = append(keys, syskeys...)

	var values = parent.creator.Driver.First_(_db, accKeys)
	if values == nil {
		return *parent.dbChild
	}

	child.AddDBVal(keys, syskeys, values)

	for _, relation := range parent.with {
		parent.CheckingRelation(child, relation)
	}

	if parent.route != "" {
		parent.addRoutes([]T{child})
	}

	return child
}

// 1. Definer et interface der matcher den signatur du leder efter
type Loader interface {
	Load()
	LoadMany(parents []ISystemFields, relationkey string)
}

func (parent *ParentDB[T]) CheckingRelation(child T, relationkey string) {
	childValue := reflect.ValueOf(child)

	if companyMethod := childValue.MethodByName(relationkey); companyMethod.IsValid() {
		// Kald Company() via reflection (da vi måske ikke kender typen på child)
		results := companyMethod.Call(nil)
		if len(results) == 0 {
			return
		}

		// 2. Tag fat i returværdien som en almindelig 'any' (interface{})
		rawResult := results[0].Interface()

		// 3. Brug Type Assertion til at tjekke for Load-metoden
		if loader, ok := rawResult.(Loader); ok {
			// 4. Kald metoden direkte (lyn hurtigt, ingen reflection her!)
			loader.Load()
		}
	}
}

func (parent *ParentDB[T]) CheckingRelationForMany(childs []ISystemFields, relationkey string) {
	child := *parent.dbChild
	childValue := reflect.ValueOf(child)

	if companyMethod := childValue.MethodByName(relationkey); companyMethod.IsValid() {
		// Kald Company() via reflection (da vi måske ikke kender typen på child)
		results := companyMethod.Call(nil)
		if len(results) == 0 {
			return
		}

		// 2. Tag fat i returværdien som en almindelig 'any' (interface{})
		rawResult := results[0].Interface()

		// 3. Brug Type Assertion til at tjekke for Load-metoden
		if loader, ok := rawResult.(Loader); ok {
			// 4. Kald metoden direkte (lyn hurtigt, ingen reflection her!)
			loader.LoadMany(childs, relationkey)
		}
	}
}

func (parent *ParentDB[T]) Get() Collection[T] {

	child := (*parent.dbChild).DB(parent.ctx)
	var keys = child.GetKeys()
	var syskeys = child.Systemcolumns()
	var accKeys = append(keys, syskeys...)

	var allvalues = parent.creator.Driver.Get_(parent.DbConn(), accKeys)
	var mylist = []T{}
	result := make([]ISystemFields, len(allvalues))

	for i, values := range allvalues {

		var object = (*parent.dbChild).DB(parent.ctx)
		object.AddDBVal(keys, syskeys, values)
		result[i] = object
		mylist = append(mylist, object)
	}

	for _, value := range parent.with {
		parent.CheckingRelationForMany(result, value)
	}

	if parent.route != "" {
		parent.addRoutes(mylist)
	}

	return mylist
}

func (parent *ParentDB[T]) Update() error {

	child := *parent.dbChild
	// Validate input
	if !parent.ignorevalidation {
		if err, errorMessage := validation.Validate(*parent.dbChild); err {
			return errors.New(errorMessage)
		}
	}

	var _db = parent.DbConn()
	var customColumns = (*parent.dbChild).GetKeys() // custom columns
	var values = []any{}                            // custom columns + updated_At

	for _, v := range customColumns {
		value, _ := child.Value(v)
		values = append(values, value)
	}
	customColumns = append(customColumns, "updated_at")
	values = append(values, time.Now())
	var id = child.PrimaryKey()
	parent.creator.Driver.Where_("id", []any{id}).Update_(_db, customColumns, values)

	return nil

}

func (parent *ParentDB[T]) Delete() error {

	child := *parent.dbChild
	var primaryKey = child.PrimaryKey()
	var _db = parent.DbConn()
	var err = parent.creator.Driver.Delete_(_db, primaryKey)

	if err != nil {
		log.Fatal(err.Error())
		return err
	}
	return nil
}

func (parent *ParentDB[T]) Pagination(r *http.Request, perPage int) map[string]any {
	pagination := Pagination{
		PageStr: r.URL.Query().Get("page"),
		PerPage: perPage,
		Path:    r.URL.Path,
	}
	parent.Limit(perPage)
	parent.OffSet(pagination.AlreadySeen())
	results := parent.Get()

	return pagination.ToMap(results.ToJson())
}

func (parent *ParentDB[T]) GetCtx() context.Context {
	return parent.ctx
}

/*
* Setting up model to use default db connection defined in env
 */
func CreateORM[T IDB[T]](ctx context.Context, model T) *ParentDB[T] {
	return createParent(ctx, model, drivers.DefaultDBConnection(ctx))
}

/*
*  Allow the model to connect to another and default database in runtime
 */
func CreateORMWithCustomDB[T IDB[T]](ctx context.Context, model T, dbCreator DBCreator) *ParentDB[T] {
	return createParent(ctx, model, dbCreator)
}

/*
* Create the model
 */
func createParent[T IDB[T]](ctx context.Context, model T, dbCreator DBCreator) *ParentDB[T] {
	dbCreator.Driver.SetTable(model.GetTable())
	model.SetSelf(func() IRepository {
		var newModel = model.DB(ctx)
		newModel.SetDBConnection(dbCreator)
		newModel.With(model.GetWith()...)
		return newModel
	})
	model.SetDBConnection(dbCreator)
	return &ParentDB[T]{creator: dbCreator, dbChild: &model, ctx: ctx}
}

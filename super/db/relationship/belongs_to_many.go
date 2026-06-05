package relationship

import (
	"errors"

	"github.com/nicklasjeppesen/going_internal/super/collections"
	. "github.com/nicklasjeppesen/going_internal/super/db/types"
)

type BelongsToManyRelation[T IDBConnection[T]] struct {
	pivotTable    string
	Holder        T      // The relationship DB
	localKey      string // Local key in foreign tabel
	foreignKey    string // ForeignKey, ex. tabel user has company_id, then company_id foreignKey
	superMapper   func(T)
	pivotsColumns []string
	primaryId     any
	relation      IRepository
	//relationToEntiy IRepository
	callerMeethod string
}

func (belong *BelongsToManyRelation[T]) ForeignKey(column string) IRelationship {
	belong.foreignKey = column
	return belong
}

func (belong *BelongsToManyRelation[T]) LocalKey(column string) IRelationship {
	belong.localKey = column
	return belong
}

func (belong *BelongsToManyRelation[T]) Pivots(column ...string) IRelationship {
	belong.pivotsColumns = column
	return belong
}

func (belong *BelongsToManyRelation[T]) PivotTable(table string) *BelongsToManyRelation[T] {
	belong.pivotTable = table
	return belong
}

func (belong *BelongsToManyRelation[T]) PivotCols(column ...string) *BelongsToManyRelation[T] {
	belong.pivotsColumns = column
	return belong
}

func (belong *BelongsToManyRelation[T]) GetName() string {
	return belong.Holder.GetName()
}

func (belong *BelongsToManyRelation[T]) Items() collections.Collection[T] {
	if relation := belong.relation.GetRelationshipHolder(belong.callerMeethod); len(relation) != 0 {
		if collec, err := relation[0].(collections.Collection[T]); err == true {
			return collec
		}
	}
	return nil
}

func (belong *BelongsToManyRelation[T]) Load() {

	var pivotsmap = belong.getPivotMap([]any{belong.relation.PrimaryKey()})
	if len(pivotsmap) == 0 {
		return
	}

	var relationsObjects = belong.getRelations(pivotsmap)[belong.relation.PrimaryKey()]
	belong.relation.SetRelationshipHolder(belong.callerMeethod, relationsObjects)
}

// relationHolder: define the object with belongs to many relationships
// strategy:
// 1: create a map: of the concret value: of its ids in pivot and value of its object, first.
// 2: Then Get all the pivots rows, and store them in a map, where relationship id i the key, the the row is the value.
// 3: Then Get all the relationships objects.
//   - When we loop through all the relations objects: we look up the ids' i the intermedia map, for relationshipHolder Id,
//   - and then we look up the relationShipholder id in the first map, to get the object, and store the relation there.
func (belong *BelongsToManyRelation[T]) LoadMany(relationee []ISystemFields, relationkey string) {

	var pivots = belong.getPivotValues(relationee)
	if len(pivots) == 0 {
		return
	}

	// map of relationee id as key, and relations object as value
	Relations := belong.getRelations(pivots)

	// set each relation to its relationee's relationship holder
	for _, parent := range relationee {
		if result, ok := Relations[parent.PrimaryKey()]; ok {
			parent.SetRelationshipHolder(relationkey, result)
		}
	}
}

// return a map of pivots values
func (belong *BelongsToManyRelation[T]) getPivotValues(relationHolders []ISystemFields) []map[string]any {

	// Step 1: Get a list of foreign ids
	var relationHolderIds = make([]any, len(relationHolders)) // list of foreign values

	for index, relationHolder := range relationHolders {
		var relationHolderId = relationHolder.PrimaryKey()
		relationHolderIds[index] = relationHolderId
	}

	// step 2: Get the pivots values, creating a array of foreign values, and a map.
	return belong.getPivotMap(relationHolderIds)
}

func (belong *BelongsToManyRelation[T]) getPivotMap(relationHolderIds []any) []map[string]any {

	// step 1: Get the pivots values, creating a array of foreign values, and a map.
	pivotsResults := belong.pivotsResults(relationHolderIds)

	// step 3: create a pivot map and return it
	return belong.createPivotMap(pivotsResults)

}

// execute SQL to get pivots values from pivot table
func (belong *BelongsToManyRelation[T]) pivotsResults(ids []any) [][]any {

	pivotDriver := belong.Holder.DB(belong.Holder.GetCtx()).GetDriver()
	pivotDriver.SetTable(belong.pivotTable)

	pivotsResults := pivotDriver.
		WhereIn_(belong.localKey, ids).
		Get_(belong.Holder.DbConn(), belong.pivotColumns())

	return pivotsResults
}

// Get columns for the pivot tables
func (belong *BelongsToManyRelation[T]) pivotColumns() []string {
	var systemColumns = []string{"id", belong.localKey, belong.foreignKey}
	var accKeys = append(systemColumns, belong.pivotsColumns...)
	return accKeys
}

// Create a pivot map from the result of the pivot
func (belong *BelongsToManyRelation[T]) createPivotMap(results [][]any) []map[string]any {

	pivotMap := make([]map[string]any, len(results))
	for index, pivot := range results {
		_pivotMap := make(map[string]any)
		for i, column := range belong.pivotColumns() {
			_pivotMap[column] = *pivot[i].(*any)
		}
		pivotMap[index] = _pivotMap
	}
	return pivotMap

}

func (belong *BelongsToManyRelation[T]) getRelations(pivots []map[string]any) map[any]collections.Collection[T] {
	relationsIds := make([]any, len(pivots))
	var relationMap = make(map[any][]map[string]any, len(pivots))

	for index, pivot := range pivots {
		var foreignValue = pivot[belong.foreignKey]
		relationsIds[index] = foreignValue
		relationMap[foreignValue] = append(relationMap[foreignValue], pivot)
	}

	// Step 3: get the relationships value, and track bag all the values, and set the relationship holder.
	var resultsSet = map[any]collections.Collection[T]{}
	var relationObjects = belong.Holder.WhereIn(belong.Holder.PrimaryKeyName(), relationsIds).Get()
	for _, relation := range relationObjects { // chats

		var relationshipHolderIds = relationMap[relation.PrimaryKey()]
		for _, relationshipHolderId := range relationshipHolderIds { // for users

			for _, pivotColumn := range belong.pivotsColumns {
				relation.SetPivotsValue(pivotColumn, relationshipHolderId[pivotColumn])
			}

			var localkey = relationshipHolderId[belong.localKey]
			tmp := resultsSet[localkey]
			tmp = tmp.Add(relation)
			resultsSet[localkey] = tmp

		}
	}
	return resultsSet
}

type PivotData map[string]any

// Takes two types of inputs:
// First: single type of foreigns id, like (1,2,3,4)
// second: a Map of Id's and pivot values map[string]PivotData
func (belong *BelongsToManyRelation[T]) Attach(args ...any) error {
	var err error
	for _, arg := range args {
		switch v := arg.(type) {

		case string, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
			belong.attach(map[any]map[string]any{v: nil})

		case PivotData:
			err = errors.New("Not allowed type, pissing foregeign id")

		case map[any]PivotData:
			belong.Attach(v)

		default:
			err = errors.New("Uknown type")
		}
	}
	return err
}

// Saving relationships.
//
// Attach a relation to many
func (belong *BelongsToManyRelation[T]) attach(input map[any]map[string]any) {
	pivotDriver := belong.Holder.DB(belong.Holder.GetCtx()).GetDriver()
	pivotDriver.SetTable(belong.pivotTable)

	for foreignId, pivotMap := range input {
		var systemColumns = []string{belong.localKey, belong.foreignKey}
		id := belong.relation.PrimaryKey()
		var values = []any{id, foreignId}

		belong.Holder.Systemcolumns()

		for pivotKey, pivotValue := range pivotMap {
			systemColumns = append(systemColumns, pivotKey)
			values = append(values, pivotValue)
		}
		pivotDriver.Save_(belong.Holder.DbConn(), systemColumns, values, nil)
	}
}

func (belong *BelongsToManyRelation[T]) Detach(a ...any) error {
	pivotDriver := belong.Holder.DB(belong.Holder.GetCtx()).GetDriver()
	pivotDriver.SetTable(belong.pivotTable)

	pivotDriver.Where_(belong.localKey, []any{belong.relation.PrimaryKey()})
	for _, val := range a {
		pivotDriver.Where_(belong.foreignKey, []any{val})
	}
	pivotDriver.Delete_(belong.Holder.DbConn(), nil)

	return nil
}

// Updating exisiting pivot values.
//   - ex.
//   - user.DB().Where("id", 1).First().Chat().UpdateExistingPivot(map[any]map[string]any{
//     1: map[string]any{"tt": "New value"}}) //Attach(1)

// ex. 2 :
// user.DB().Where("id", 1).First().Chat().UpdateExistingPivot(func() map[any]map[string]any {
// user.DB().First() // Do more stuff
// return map[any]map[string]any{"T": nil}
// }())
func (belong *BelongsToManyRelation[T]) UpdateExistingPivot(input map[any]map[string]any) {

	for foreignId, pivotMap := range input {
		pivotDriver := belong.Holder.DB(belong.Holder.GetCtx()).GetDriver()
		pivotDriver.SetTable(belong.pivotTable)

		pivotDriver.
			Where_(belong.localKey, []any{belong.primaryId}).
			Where_(belong.foreignKey, []any{foreignId})

		var systemColumns = []string{}
		var values = []any{}

		for pivotKey, pivotValue := range pivotMap {
			systemColumns = append(systemColumns, pivotKey)
			values = append(values, pivotValue)
		}

		pivotDriver.
			Update_(belong.Holder.DbConn(), systemColumns, values)
	}
}

func NewBelongsToMany[T IDBConnection[T]](current T, relationToEntiy IRepository) *BelongsToManyRelation[T] {

	var relation = BelongsToManyRelation[T]{
		Holder:        current,
		primaryId:     current.PrimaryKey(),
		relation:      relationToEntiy,
		callerMeethod: CallerMethodName(),
	}

	// Create basis object
	parentTable := removeTrailingS(relationToEntiy.GetTable())
	childParentTable := removeTrailingS(current.GetTable())
	relation.pivotTable = PivotTableName(childParentTable, parentTable)
	relation.localKey = parentTable + "_id"
	relation.foreignKey = childParentTable + "_id"

	return &relation
}

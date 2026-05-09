package relationship

import (
	"fmt"
	. "myapp/internal/super/db/types"
)

type BelongsTo[T IDBConnection[T]] struct {
	Holder          T // The relationship DB
	pivotTable      string
	localKey        string // Local key in foreign tabel
	foreignKey      string // ForeignKey, ex. tabel user has company_id, then company_id foreignKey
	pivotsColumns   []string
	primaryId       any
	relation        ISystemFields
	relationToEntiy IRepository
	callerMeethod   string
}

func (belong *BelongsTo[T]) Attach(relation IRepository) error {
	return belong.relation.SetValue(belong.foreignKey, relation.PrimaryKey())
}

func (belong *BelongsTo[T]) Save(relation IRepository) error {
	panic("Method not implemented")
}

// Set det local Key, determine by its name + "_id"
// Ex. user table might hav the column company_id, which is the foreign key
// Sets the local key for the relationship
func (belong *BelongsTo[T]) ForeignKey(column string) IRelationship {
	belong.foreignKey = column
	return belong
}

/*
 * Asume a table follow the convension of id as primary key, if other key is used, then set it here
 */
func (belong *BelongsTo[T]) LocalKey(column string) IRelationship {
	belong.localKey = column
	return belong
}

/*
* Set the The foreign key based on the table name
 */
func (belong *BelongsTo[T]) setparent(relation ISystemFields) {
	if belong.foreignKey != "" {
		return
	}
	belong.foreignKey = removeTrailingS(belong.Holder.GetTable()) + "_id"
	belong.relation = relation
}

func (belong *BelongsTo[T]) Item() T {
	if relation := belong.relationToEntiy.GetRelationshipHolder(belong.callerMeethod); relation != nil {
		return relation[0].(T)
	}
	return belong.Holder
}

func (belong *BelongsTo[T]) Load() {
	parent := belong.relationToEntiy
	belong.setparent(parent) // Setting Foreign key
	// Run SELECT query
	var keys = belong.Holder.GetKeys()
	var syskeys = belong.Holder.Systemcolumns()
	var accKeys = append(keys, syskeys...)
	var defaultConn = belong.Holder.DBConnection()

	dbconn := belong.Holder.DBConnection().Driver.Open(defaultConn.ConnectionString)
	defer dbconn.Close()

	var foreignKeyValue, _ = parent.Value(belong.foreignKey)
	var driver = belong.Holder.DBConnection().Driver
	driver.SetTable(belong.Holder.GetTable())
	var values = driver.Where_(belong.localKey, []any{foreignKeyValue}).First_(dbconn, accKeys)
	belong.Holder.AddDBVal(keys, syskeys, values)

	parent.SetRelationshipHolder(belong.callerMeethod, belong.Holder)
}

// Strategi: vi går tilbage til det som var før, men, her laver jeg en liste og et map af localids,
// Hvor idder og selve værdier er i en map,
// Når værdierne er fået fra get, så loppes de igennem, og tilføjes til lokale værdier og gennems på den måde.
// Køretid: O(N * 2)
func (belong *BelongsTo[T]) LoadMany(parents []ISystemFields, relationkey string) {

	belong.setparent(parents[0]) // Setting Foreign key

	var localIds = make([]any, len(parents)) // list of foreign values
	var localHolder = make(map[any][]ISystemFields, len(parents))
	for index, parent := range parents {

		var localId, _ = parent.Value(belong.foreignKey)
		localIds[index] = localId

		localIdString := fmt.Sprintf("%v", localId)
		localHolder[localIdString] = append(localHolder[localIdString], parent)
	}

	var keys = belong.Holder.GetKeys()
	var syskeys = belong.Holder.Systemcolumns()
	var accKeys = append(keys, syskeys...)
	var defaultConn = belong.Holder.DBConnection()

	dbconn := belong.Holder.DBConnection().Driver.Open(defaultConn.ConnectionString)
	defer dbconn.Close()

	var driver = belong.Holder.DBConnection().Driver
	driver.SetTable(belong.Holder.GetTable())

	var values = driver.WhereIn_(belong.localKey, localIds).Get_(dbconn, accKeys)

	var relationships = make([]ISystemFields, len(values))
	for i := range relationships {

		temp := belong.Holder.CopySelf()
		temp.AddDBVal(keys, syskeys, values[i])
		relationships[i] = temp
	}

	for _, company := range relationships {
		var relation_id, _ = company.Value(belong.localKey)
		if relation_id == nil {
			continue
		}
		var relationIdString = fmt.Sprintf("%v", relation_id)
		var _relation, ok = localHolder[relationIdString]
		if ok {
			for _, value := range _relation {
				value.SetRelationshipHolder(relationkey, company)
			}
		}
	}
}

func NewBelongsTo[T IDBConnection[T]](currentEntity T, RelationToEntiy IRepository) *BelongsTo[T] {
	return &BelongsTo[T]{
		Holder:          currentEntity,
		localKey:        "id",
		relationToEntiy: RelationToEntiy,
		callerMeethod:   CallerMethodName(),
	}

}

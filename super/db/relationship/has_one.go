package relationship

import (
	"fmt"

	. "github.com/nicklasjeppesen/going_internal/super/db/types"
)

type HasOneRelation[T IDB[T]] struct {
	Holder          T
	localKey        string // Local key in foreign table
	foreignKey      string // foreign key, e.g. table user has company_id, then company_id is the foreignKey
	relation        ISystemFields
	relationToEntiy IRepository
	callerMeethod   string
}

func (belong *HasOneRelation[T]) Save(data IRepository) error {
	data.SetValue(belong.foreignKey, belong.Holder.PrimaryKey())
	_, err := data.SaveNonGenerics()
	return err
}

func (belong *HasOneRelation[T]) GetHolder() ISystemFields {
	return belong.Holder
}

// Set the local key, determined by its name + "_id"
// Ex. user table might hav the column company_id, which is the foreign key
// Sets the local key for the relationship
func (belong *HasOneRelation[T]) ForeignKey(column string) IRelationship {
	belong.foreignKey = column
	return belong
}

// Asume a table follow the convension of id as primary key, if other key is used, then set it here
func (belong *HasOneRelation[T]) LocalKey(column string) IRelationship {
	belong.localKey = column
	return belong
}

func (belong *HasOneRelation[T]) setparent(relation ISystemFields) {
	if belong.foreignKey != "" {
		return
	}
	belong.foreignKey = removeTrailingS(relation.GetTable()) + "_id"
	belong.relation = relation
}

func (belong *HasOneRelation[T]) GetName() string {
	return belong.GetName()
}

func (belong *HasOneRelation[T]) Item() T {
	if relation := belong.relationToEntiy.GetRelationshipHolder(belong.callerMeethod); relation != nil {
		if len(relation) == 0 {
			return belong.Holder
		}
		return relation[0].(T)
	}
	return belong.Holder
}

func (belong *HasOneRelation[T]) Load() {

	belong.setparent(belong.relationToEntiy) // Setting Foreign key
	var localKeyValue, _ = belong.relationToEntiy.Value(belong.localKey)
	if localKeyValue == nil {
		return
	}
	first := belong.Holder.Where(belong.foreignKey, localKeyValue).First()
	belong.relationToEntiy.SetRelationshipHolder(belong.callerMeethod, first)

}

// Strategy: build a list and a map of the local ids,
// where the ids and the values themselves live in a map.
// Once the values have been fetched with Get, they are looped over and attached to the local values that way.
// Running time: O(N * 2)
func (belong *HasOneRelation[T]) LoadMany(parents []ISystemFields, relationkey string) {
	belong.setparent(parents[0])
	var localIds = make([]any, len(parents)) // list of foreign values
	var localHolder = make(map[string]ISystemFields, len(parents))
	for index, parent := range parents {
		var localId, _ = parent.Value(belong.localKey)
		localIds[index] = fmt.Sprint(localId)
		localHolder[fmt.Sprint(localId)] = parent
	}
	var relationships = belong.Holder.WhereIn(belong.foreignKey, localIds).Get()
	for _, user := range relationships {
		var relation_id, _ = user.Value(belong.foreignKey)
		if relation_id == nil {
			continue
		}

		var _relation, ok = localHolder[fmt.Sprint(relation_id)]
		if ok {
			_relation.SetRelationshipHolder(relationkey, user)
		}
	}
}

func NewHasOne[T IDB[T]](holder T, relationToEntiy IRepository) *HasOneRelation[T] {
	return &HasOneRelation[T]{
		Holder:          holder,
		localKey:        "id",
		relationToEntiy: relationToEntiy,
		callerMeethod:   CallerMethodName(),
	}
}

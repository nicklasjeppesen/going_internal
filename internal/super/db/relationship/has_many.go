package relationship

import (
	"github.com/nicklasjeppesen/going_internal/internal/super/collections"
	. "github.com/nicklasjeppesen/going_internal/internal/super/db/types"
)

type HasManyRelation[T IDB[T]] struct {
	Holder          T
	localKey        string
	foreignKey      string
	relation        ISystemFields
	relationToEntiy IRepository
	callerMeethod   string
}

func (belong *HasManyRelation[T]) Attach(data IRepository) error {

	data.SetValue(belong.foreignKey, belong.Holder.PrimaryKey())
	_, err := data.SaveNonGenerics()
	return err

}

func (belong *HasManyRelation[T]) Save(data IRepository) error {
	data.SetValue(belong.foreignKey, belong.Holder.PrimaryKey())
	_, err := data.SaveNonGenerics()
	return err

}

// Set det local Key, determine by its name + "_id"
// Ex. user table might hav the column company_id, which is the foreign key
// Sets the local key for the relationship
func (belong *HasManyRelation[T]) ForeignKey(column string) IRelationship {
	belong.foreignKey = column
	return belong
}

// Asume a table follow the convension of id as primary key, if other key is used, then set it here

func (belong *HasManyRelation[T]) LocalKey(column string) IRelationship {
	belong.localKey = column
	return belong
}

func (belong *HasManyRelation[T]) setparent(relation ISystemFields) {

	if belong.foreignKey != "" {
		return
	}
	belong.foreignKey = removeTrailingS(relation.GetTable()) + "_id"
	belong.relation = relation
}

func (belong *HasManyRelation[T]) Load() {
	belong.setparent(belong.relationToEntiy) // Setting Foreign key

	var localKeyValue, _ = belong.relationToEntiy.Value(belong.localKey)
	if localKeyValue == nil {
		return
	}
	response := belong.Holder.Where(belong.foreignKey, localKeyValue).Get()
	belong.relationToEntiy.SetRelationshipHolder(belong.callerMeethod, response)
}

func (belong *HasManyRelation[T]) Items() collections.Collection[T] {
	if relation := belong.relationToEntiy.GetRelationshipHolder(belong.callerMeethod); len(relation) > 0 {
		results := relation[0].(collections.Collection[T])
		return results
	}
	return nil
}

// Strategi: vi går tilbage til det som var før, men, her laver jeg en liste og et map af localids,
// Hvor idder og selve værdier er i en map,
// Når værdierne er fået fra get, så loppes de igennem, og tilføjes til lokale værdier og gennems på den måde.
// Køretid: O(N * 2)
func (belong *HasManyRelation[T]) LoadMany(parents []ISystemFields, relationkey string) {
	belong.setparent(parents[0]) // Setting Foreign key

	var localIds = make([]any, len(parents))                      // List of company id's
	var localHolder = make(map[any][]ISystemFields, len(parents)) // key = company id, value: companies

	for index, parent := range parents {
		var localId, _ = parent.Value(belong.localKey)
		localIds[index] = localId
		localHolder[localId] = append(localHolder[localId], parent)
	}

	var resultsSet = map[any]collections.Collection[T]{}
	var relationships = belong.Holder.WhereIn(belong.foreignKey, localIds).Get() // users
	for _, user := range relationships {
		var relation_id, _ = user.Value(belong.foreignKey) // User primary id
		if relation_id == nil {
			continue
		}

		if _relations, ok := localHolder[relation_id]; ok {
			for _, _relation := range _relations {
				tmp := resultsSet[_relation.PrimaryKey()]
				tmp = tmp.Add(user)
				resultsSet[_relation.PrimaryKey()] = tmp

			}
		}
	}
	for _, parent := range parents {
		if result, ok := resultsSet[parent.PrimaryKey()]; ok {
			parent.SetRelationshipHolder(relationkey, result)
		}
	}
}

func NewHasMany[T IDB[T]](holder T, relationToEntiy IRepository) *HasManyRelation[T] {
	return &HasManyRelation[T]{
		Holder:          holder,
		localKey:        "id",
		relationToEntiy: relationToEntiy,
		callerMeethod:   CallerMethodName(),
	}
}

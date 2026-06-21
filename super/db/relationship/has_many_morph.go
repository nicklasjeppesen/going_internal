package relationship

import (
	"github.com/nicklasjeppesen/going_internal/super/collections"
	. "github.com/nicklasjeppesen/going_internal/super/db/types"
)

type HasManyMorphRelation[T IDB[T]] struct {
	Holder          T
	localKey        string // Local key in foreign tabel
	foreignKey      string // fremmed nøgle ex. tabel user has company_id, så er company_id foreignKey
	relation        ISystemFields
	morph           string
	relationToEntiy IRepository
	callerMeethod   string
}

// ex. company have comments,
func (belong *HasManyMorphRelation[T]) Attach(data T) error {

	data = data.DB(belong.Holder.GetCtx())

	// Set relation type id
	foreignId := belong.morph + "_id"
	key := belong.relationToEntiy.PrimaryKey()
	data.SetValue(foreignId, key)

	// Set foreign relation type key
	foreignName := belong.morph + "_type"
	foreignType := belong.relationToEntiy.GetName()
	data.SetValue(foreignName, foreignType)

	// Save record
	_, err := data.SaveNonGenerics()
	return err

}

func (belong *HasManyMorphRelation[T]) Save(data IRepository) error {
	data.SetValue(belong.foreignKey, belong.Holder.PrimaryKey())
	_, err := data.SaveNonGenerics()
	return err

}

// Set det local Key, determine by its name + "_id"
// Ex. user table might hav the column company_id, which is the foreign key
// Sets the local key for the relationship
func (belong *HasManyMorphRelation[T]) ForeignKey(column string) *HasManyMorphRelation[T] {
	belong.foreignKey = column
	return belong
}

// Asume a table follow the convension of id as primary key, if other key is used, then set it here

func (belong *HasManyMorphRelation[T]) LocalKey(column string) *HasManyMorphRelation[T] {
	belong.localKey = column
	return belong
}

func (belong *HasManyMorphRelation[T]) setparent(relation ISystemFields) {
	if belong.foreignKey != "" {
		return
	}
	belong.foreignKey = removeTrailingS(relation.GetTable()) + "_id"
	belong.relation = relation
}

func (belong *HasManyMorphRelation[T]) Load() {
	belong.setparent(belong.relationToEntiy) // Setting Foreign key
	foreignName := belong.morph + "_type"
	foreIgnId := belong.morph + "_id"

	var localKeyValue, _ = belong.relationToEntiy.Value(belong.localKey)
	if localKeyValue == nil {
		return
	}
	response := belong.Holder.Where(foreignName, belong.relationToEntiy.GetName()).
		Where(foreIgnId, localKeyValue).Get()
	belong.relationToEntiy.SetRelationshipHolder(belong.callerMeethod, response)
}

func (belong *HasManyMorphRelation[T]) Items() collections.Collection[T] {
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
func (belong *HasManyMorphRelation[T]) LoadMany(parents []ISystemFields, relationkey string) {
	belong.setparent(parents[0]) // Setting Foreign key

	var localIds = make([]any, len(parents))                      // List of company id's
	var localHolder = make(map[any][]ISystemFields, len(parents)) // key = company id, value: companies

	for index, parent := range parents {
		var localId, _ = parent.Value(belong.localKey)
		localIds[index] = localId
		localHolder[localId] = append(localHolder[localId], parent)
	}

	foreignName := belong.morph + "_type"
	foreIgnId := belong.morph + "_id"

	var relationships = belong.Holder.
		Where(foreignName, parents[0].GetName()).
		WhereIn(foreIgnId, localIds).Get() // users

	var resultsSet = map[any]collections.Collection[T]{}
	for _, user := range relationships {
		var relation_id, _ = user.Value(foreIgnId) // User primary id
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

func NewHasManyMorph[T IDB[T]](holder T, relationToEntiy IRepository) *HasManyMorphRelation[T] {
	return &HasManyMorphRelation[T]{
		Holder:          holder,
		localKey:        "id",
		morph:           holder.GetName() + "able",
		relationToEntiy: relationToEntiy,
		callerMeethod:   CallerMethodName(),
	}
}

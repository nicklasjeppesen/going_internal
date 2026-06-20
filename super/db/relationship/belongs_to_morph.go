package relationship

import (
	"fmt"

	. "github.com/nicklasjeppesen/going_internal/super/db/types"
)

/*
*
* BelongsToMorph relation: a model can belong to more models
* This works by having the model to have two column, a type an id column for defining the model of of owner and foreign id.
*

  - Example of used:
    Its recommendated to define an interface that include Ire

//* The interface to be implemented by models that can be entryable

	type IEntry interface {
		IRepository // Required interface to creating the interface.
		//
		// Abstract function
		// needed to implemnt delegatable models

		Title() string

		PrintHi() string
	}

//* The struct that can be Implemented into the struct in models to make them delegatable entries
type EntryAble struct {
}

	func (e *EntryAble) PrintHi() string {
		return "Hi from EntryAble"
	}

//* ----- The DB MODEL ----------------------------------------------- //

	type Entry struct {
		ActiveRecord[*Entry] //int, created_at, updated_at
		Name                 string
		EntryAble_type       string
		EntryAble_id         int64
		EntryAble            IEntry // Value holder
	}

	const delegate = "entryable"
	var (
		entryAbles = []IEntry{
			new(Company).DB(),
			new(User).DB().With("company"),
		}
	)

	func (e Entry) DB() *Entry {
		entry := &Entry{}
		entry.Table = "entries"
		entry.Columns = Columns{
			"name":             &entry.Name,
			delegate + "_id":   &entry.EntryAble_id,
			delegate + "_type": &entry.EntryAble_type,
		}
		entry.ParentDB = CreateORM(entry)
		return entry
	}
	//* ------------------Relations --------------------------------//
	func (e *Entry) Relations() IRelationships {
		return e.CreateRelationShip(IRelationships{
			"entryable": BelongsToMorph(entryAbles, &e.EntryAble, delegate),
		})
	}

	//* ----------- Scopes ----------------------------------------//
	func (e *Entry) Company() IDB[*Entry] {
		return e.WhereMorph(delegate, new(Company).DB().GetName())
	}
	func (e *Entry) User() IDB[*Entry] {
		return e.WhereMorph(delegate, new(User).DB().GetName())
	}
	}

	type Company struct {
		ActiveRecord[*Company]
		Name  string

		// DelegateAbles
		EntryAble // include the EntryAble struct to make company delegatable
	}

	func (c Company) DB() *Company {

		company := &Company{}
		company.Table = "companies"
		company.Columns = Columns{
			// Column		  "values"
			"name": &company.Name,
		}
		company.ParentDB = CreateORM(company)
		return company
	}

	// -------------- EntryAble functions --------------------//

	//* Implement the abstract method
	func (company *Company) Title() string {
		return company.Name + ", Company Title is called"
	}
	//* Optional: override the PritHi function
	func (company *Company) PrintHi() string {
		return company.EntryAble.PrintHi() + " with added logic to it from Company"
	}

	//* -------------- Usage ----------------------------//

	var entries = models.Entry{}.DB().With("entryable").OrderByDesc("id").Company().Get()
	for _, entry := range entries {
		fmt.Println(entry.EntryAble.Title())
		fmt.Println(entry.EntryAble.PrintHi())
		fmt.Println(entry.EntryAble.(*Company).Name)

	}
*/
type BelongsToMorphRelation[T IRepository] struct {
	localKey        string // Local key in foreign tabel
	foreignKey      string // fremmed nøgle ex. tabel user has company_id, så er company_id foreignKey
	morph           string
	relation        ISystemFields
	relations       func(relationName string) IRepository
	relationToEntiy IRepository
	callerMeethod   string
}

func (belong *BelongsToMorphRelation[T]) Attach(parent IRepository) error {
	belong.relation.SetValue(belong.morph+"_type", parent.GetName())
	belong.relation.SetValue(belong.morph+"_id", parent.PrimaryKey())
	return nil
}

// Set det local Key, determine by its name + "_id"
// Ex. user table might hav the column company_id, which is the foreign key
// Sets the local key for the relationship
func (belong *BelongsToMorphRelation[T]) ForeignKey(column string) IRelationship {
	belong.foreignKey = column
	return belong
}

// Assume a table follow the convension of id as primary key, if other key is used, then set it here
func (belong *BelongsToMorphRelation[T]) LocalKey(column string) IRelationship {
	belong.localKey = column
	return belong
}

func (belong *BelongsToMorphRelation[T]) Load() {
	parent := belong.relationToEntiy
	ForeignTypeName, _ := parent.Value(belong.morph + "_type")
	ForeignTypeId, _ := parent.Value(belong.morph + "_id")
	modeltemplate := belong.relations(ForeignTypeName.(string)).CopySelf()
	var model = modeltemplate.WhereNonGeneric(belong.localKey, ForeignTypeId).FirstNonGeneric()
	parent.SetRelationshipHolder(belong.callerMeethod, model)
}

func (belong *BelongsToMorphRelation[T]) Item() T {
	if relation := belong.relationToEntiy.GetRelationshipHolder(belong.callerMeethod); len(relation) > 0 {
		return relation[0].(T)
	} else {
		value, _ := belong.relationToEntiy.Value("entryable_type")
		id, _ := belong.relationToEntiy.Value("entryable_id")
		fmt.Println("No relation found for type:", value, " with id:", id)
	}
	return *new(T)
}

// 1. Create a map key: ForeignType_id, value: array of ids for this type.
// 2. Foreach type, Call it by a get. and get its input
// 3. Create the specicis type of them, så Get return [][] make it a user model eks.
// 4. Then find the parents with the matching type and id, get the relation by relationkey, and call its mapper function, to set the value
func (belong *BelongsToMorphRelation[T]) LoadMany(parents []ISystemFields, relationkey string) {

	var ForeignKeysMap, ModelKeys = belong.getForeignKeyMap(parents)

	// 2. Foreach type, Call it by a get. and get its input
	for modelName, foreignKeys := range ForeignKeysMap {
		// 3. Create the specicis type of them, så Get return [][] make it a user model eks.

		// OBS: Check for null
		modelTemplate := belong.relations(modelName)
		if modelTemplate == nil {
			continue
		}

		models := modelTemplate.WhereInNonGeneric(belong.localKey, foreignKeys).GetNonGeneric()

		//4. Then find the parents with the matching type and id, get the relation by relationkey, and call its mapper function, to set the value
		for _, model := range models {
			localKey, _ := model.Value(belong.localKey)
			modelKey := modelName + "_" + fmt.Sprint(localKey)
			for _, modelKey := range ModelKeys[modelKey] {
				modelKey.SetRelationshipHolder(relationkey, model)
			}
		}
	}
}

func (belong *BelongsToMorphRelation[T]) getMorphTypeAndId(parent ISystemFields) (string, string) {
	ForeignTypeName_, _ := parent.Value(belong.morph + "_type")
	ForeignTypeId_, _ := parent.Value(belong.morph + "_id")
	ForeignTypeName := fmt.Sprint(ForeignTypeName_)
	ForeignTypeId := fmt.Sprint(ForeignTypeId_)
	return ForeignTypeName, ForeignTypeId
}

func (belong *BelongsToMorphRelation[T]) getForeignKeyMap(parents []ISystemFields) (map[string][]any, map[string][]ISystemFields) {

	// section 1. Create a map key: ForeignType_id, value: array of ids for this type.

	/*
		* ForeignKeyMap
		* Map That hold all the foreign keys for a given model.
		* ex: [
			Company: [1,2,3,4],
			User: [1,2,3]
		]
	*/
	var ForeignKeysMap = map[string][]any{}

	/*
		* ModelKeys
		* Create a concret map for a model and a its id, it used to find the model for passing its modelKey to it:
		* ex. [
			Company_1: []ISystemFields[
				*models.Entry has delegate_type = "Company" and delegate_id = 1
				*models.Entry has delegate_type = "Company" and delegate_id = 1
				]
			User_3: []ISystemFields[
				*models.Entry has delegate_type = "User" and delegate_id = 3
				*models.Entry has delegate_type = "User" and delegate_id = 3
				],...
		*
	*/
	var ModelKeys = map[string][]ISystemFields{}

	for _, parent := range parents {
		ForeignTypeName, ForeignTypeId := belong.getMorphTypeAndId(parent)
		ForeignKeysMap[ForeignTypeName] = append(ForeignKeysMap[ForeignTypeName], ForeignTypeId)

		var ModelKey string = ForeignTypeName + "_" + ForeignTypeId
		ModelKeys[ModelKey] = append(ModelKeys[ModelKey], parent)
	}

	return ForeignKeysMap, ModelKeys
}

func NewBelongsToMorph[T IRepository](delegateAble string, relations []T, relationToEntiy IRepository) *BelongsToMorphRelation[T] {
	var GetRelation = func(relationName string) IRepository {
		for _, relation := range relations {
			if relation.GetName() == relationName {
				return relation.CopySelf() // return new instance of it.
			}
		}
		return nil
	}

	return &BelongsToMorphRelation[T]{
		localKey:        "id",
		morph:           delegateAble,
		relations:       GetRelation,
		callerMeethod:   CallerMethodName(),
		relationToEntiy: relationToEntiy,
	}
}

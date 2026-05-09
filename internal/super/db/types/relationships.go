package types

/*
*
* Map for holding relationships
 */
type IRelationships map[string]IRelationship

/*
*
* Interface for relationship types
 */
type IRelationship interface {
	/*
	* Custom fuction to set the local key
	 */
	LocalKey(column string) IRelationship

	/*
	* Custom fuction to set the foreign key
	 */
	ForeignKey(column string) IRelationship

	/*
	* Load a single relationship
	 */
	Load()
	/*
	* Load many relationships
	 */
	LoadMany(parent []ISystemFields, key string)
}

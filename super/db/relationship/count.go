package relationship

import (
	"fmt"

	. "github.com/nicklasjeppesen/going_internal/super/db/types"
)

// Counting relations for WithCount: one grouped query per relation for all
// parents (BelongsToMany needs a second one for the related rows), with the
// relation's own conditions. Every parent gets a count, 0 when it has none.

// countKey makes the values of a key comparable, whatever Go type the driver
// or the model gives them (int64 from the database, int in a model, …).
func countKey(value any) string { return fmt.Sprint(value) }

// localValues are the parents' values of localKey, without empty ones.
func localValues(parents []ISystemFields, localKey string) []any {
	values := make([]any, 0, len(parents))
	for _, parent := range parents {
		if value, _ := parent.Value(localKey); value != nil {
			values = append(values, value)
		}
	}
	return values
}

// setCounts gives each parent its count from counts (keyed by countKey of its
// localKey), 0 when it is missing.
func setCounts(parents []ISystemFields, localKey, relationkey string, counts map[string]int64) {
	for _, parent := range parents {
		value, _ := parent.Value(localKey)
		parent.SetRelationCount(relationkey, counts[countKey(value)])
	}
}

// byKey re-keys a CountBy result with countKey.
func byKey(counts map[any]int64) map[string]int64 {
	out := make(map[string]int64, len(counts))
	for key, n := range counts {
		out[countKey(key)] += n
	}
	return out
}

// CountMany counts each parent's related rows (WithCount), with the
// relation's Where/WhereIn.
func (belong *HasManyRelation[T]) CountMany(parents []ISystemFields, relationkey string) error {
	belong.setparent(parents[0])
	query := belong.Apply(belong.Holder.WhereIn(belong.foreignKey, localValues(parents, belong.localKey)))
	counts, err := query.CountBy(belong.foreignKey)
	if err != nil {
		return err
	}
	setCounts(parents, belong.localKey, relationkey, byKey(counts))
	return nil
}

// CountMany counts each parent's morph children (WithCount).
func (belong *HasManyMorphRelation[T]) CountMany(parents []ISystemFields, relationkey string) error {
	belong.setparent(parents[0])
	id := belong.morph + "_id"
	counts, err := belong.Holder.
		Where(belong.morph+"_type", parents[0].GetName()).
		WhereIn(id, localValues(parents, belong.localKey)).
		CountBy(id)
	if err != nil {
		return err
	}
	setCounts(parents, belong.localKey, relationkey, byKey(counts))
	return nil
}

// CountMany counts each parent's related rows through the pivot table
// (WithCount), with WherePivot/WherePivotIn and the relation's Where/WhereIn.
// Pivot rows whose related row is gone (or doesn't match) are not counted,
// like Items.
func (belong *BelongsToManyRelation[T]) CountMany(parents []ISystemFields, relationkey string) error {
	pkName := parents[0].PrimaryKeyName()
	pivots := belong.createPivotMap(belong.pivotsResults(localValues(parents, pkName)))

	// The related rows that exist and match the relation's conditions.
	var related []any
	seen := map[string]bool{}
	for _, pivot := range pivots {
		foreign := pivot[belong.foreignKey]
		if key := countKey(foreign); !seen[key] {
			seen[key] = true
			related = append(related, foreign)
		}
	}
	matching := map[string]int64{}
	if len(related) > 0 {
		query := belong.QueryModifiers.Apply(belong.Holder.WhereIn(belong.Holder.PrimaryKeyName(), related))
		found, err := query.CountBy(belong.Holder.PrimaryKeyName())
		if err != nil {
			return err
		}
		matching = byKey(found)
	}

	counts := map[string]int64{}
	for _, pivot := range pivots {
		local, foreign := pivot[belong.localKey], pivot[belong.foreignKey]
		if matching[countKey(foreign)] > 0 {
			counts[countKey(local)]++
		}
	}
	setCounts(parents, pkName, relationkey, counts)
	return nil
}

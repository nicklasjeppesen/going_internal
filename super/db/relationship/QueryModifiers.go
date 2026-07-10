package relationship

import (
	. "github.com/nicklasjeppesen/going_internal/super/db/types"
)

type whereCondition struct {
	column string
	values []any
}

type orderCondition struct {
	column string
	desc   bool
}

// Querymidifier contains where/whereIn/OrderBy-condition, that skal be use on the relation entity (T)
// This allow filtering and sorting to be implemented
type QueryModifiers[T IDBConnection[T]] struct {
	wheres   []whereCondition
	wheresIn []whereCondition
	orderBys []orderCondition
}

func (q *QueryModifiers[T]) Where(column string, values ...any) {
	q.wheres = append(q.wheres, whereCondition{column: column, values: values})
}

func (q *QueryModifiers[T]) WhereIn(column string, values []any) {
	q.wheres = append(q.wheresIn, whereCondition{column: column, values: values})
}

func (q *QueryModifiers[T]) OrderBy(column string) {
	q.orderBys = append(q.orderBys, orderCondition{column: column, desc: false})
}

func (belong *BelongsToManyRelation[T]) OrderByDesc(column string) *BelongsToManyRelation[T] {
	belong.orderBys = append(belong.orderBys, orderCondition{column: column, desc: true})
	return belong
}

// Apply kører de gemte betingelser på query/T og returnerer den (chainbare) query klar til .Get().
func (q *QueryModifiers[T]) Apply(query T) T {
	for _, w := range q.wheres {
		query = query.Where(w.column, w.values...)
	}

	for _, w := range q.wheresIn {
		query = query.WhereIn(w.column, w.values)
	}

	for _, o := range q.orderBys {
		if o.desc {
			query = query.OrderByDesc(o.column)
		} else {
			query = query.OrderBy(o.column)
		}
	}

	for _, o := range q.orderBys {
		if o.desc {
			query = query.OrderByDesc(o.column)
		} else {
			query = query.OrderBy(o.column)
		}
	}

	return query
}

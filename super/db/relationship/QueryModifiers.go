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
type QueryModifiers[T IDBConnection[T], R any] struct {
	self     R
	wheres   []whereCondition
	wheresIn []whereCondition
	orderBys []orderCondition
}

func (q *QueryModifiers[T, R]) newQueryModifiers(self R) {
	q.self = self
}

func (q *QueryModifiers[T, R]) Where(column string, values ...any) R {
	q.wheres = append(q.wheres, whereCondition{column: column, values: values})
	return q.self
}

func (q *QueryModifiers[T, R]) WhereIn(column string, values []any) R {
	q.wheresIn = append(q.wheresIn, whereCondition{column: column, values: values})
	return q.self
}

func (q *QueryModifiers[T, R]) OrderBy(column string) R {
	q.orderBys = append(q.orderBys, orderCondition{column: column, desc: false})
	return q.self
}

func (q *QueryModifiers[T, R]) OrderByDesc(column string) R {
	q.orderBys = append(q.orderBys, orderCondition{column: column, desc: true})
	return q.self
	//sreturn belong
}

// Apply kører de gemte betingelser på query/T og returnerer den (chainbare) query klar til .Get().
func (q *QueryModifiers[T, R]) Apply(query T) T {
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

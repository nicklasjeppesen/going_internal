package drivers

import (
	"context"
	"fmt"
	"strings"

	types "github.com/nicklasjeppesen/going_internal/super/db/types"
)

// Counting, shared by the SQLite and Postgres drivers: the query's conditions
// (Where, WhereIn, Or) are kept; ordering, limit, offset and locks are not.

// countQuery is "SELECT <what> FROM table <conditions>".
func countQuery(what, table string, conditions []string) string {
	query := fmt.Sprintf("SELECT %s FROM %s", what, table)
	if len(conditions) > 0 {
		query += strings.Join(conditions, " ")
	}
	return query
}

// count runs SELECT COUNT(*).
func count(ctx context.Context, _db types.DBTX, table string, conditions []string, params []any) (int64, error) {
	query := countQuery("COUNT(*)", table, conditions)
	var n int64
	if err := _db.QueryRowContext(ctx, query, params...).Scan(&n); err != nil {
		return 0, fmt.Errorf("count %s: %w", table, err)
	}
	return n, nil
}

// exists runs SELECT EXISTS (SELECT 1 … LIMIT 1).
func exists(ctx context.Context, _db types.DBTX, table string, conditions []string, params []any) (bool, error) {
	query := "SELECT EXISTS (" + countQuery("1", table, conditions) + " LIMIT 1)"
	var found bool
	if err := _db.QueryRowContext(ctx, query, params...).Scan(&found); err != nil {
		return false, fmt.Errorf("exists %s: %w", table, err)
	}
	return found, nil
}

// countBy runs SELECT column, COUNT(*) … GROUP BY column.
func countBy(ctx context.Context, _db types.DBTX, table, column string, conditions []string, params []any) (map[any]int64, error) {
	query := countQuery(column+", COUNT(*)", table, conditions) + " GROUP BY " + column
	rows, err := _db.QueryContext(ctx, query, params...)
	if err != nil {
		return nil, fmt.Errorf("count %s by %s: %w", table, column, err)
	}
	defer rows.Close()
	out := map[any]int64{}
	for rows.Next() {
		var key any
		var n int64
		if err := rows.Scan(&key, &n); err != nil {
			return nil, fmt.Errorf("count %s by %s: %w", table, column, err)
		}
		if b, ok := key.([]byte); ok {
			key = string(b)
		}
		out[key] = n
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("count %s by %s: %w", table, column, err)
	}
	return out, nil
}

func (parent *SQLite) Count_(_db types.DBTX) (int64, error) {
	return count(parent.ctx, _db, parent.table, parent.Conditions, parent.Params)
}

func (parent *SQLite) Exists_(_db types.DBTX) (bool, error) {
	return exists(parent.ctx, _db, parent.table, parent.Conditions, parent.Params)
}

func (parent *SQLite) CountBy_(_db types.DBTX, column string) (map[any]int64, error) {
	return countBy(parent.ctx, _db, parent.table, column, parent.Conditions, parent.Params)
}

func (parent *PostgresDB) Count_(_db types.DBTX) (int64, error) {
	return count(parent.ctx, _db, parent.table, parent.Conditions, parent.Params)
}

func (parent *PostgresDB) Exists_(_db types.DBTX) (bool, error) {
	return exists(parent.ctx, _db, parent.table, parent.Conditions, parent.Params)
}

func (parent *PostgresDB) CountBy_(_db types.DBTX, column string) (map[any]int64, error) {
	return countBy(parent.ctx, _db, parent.table, column, parent.Conditions, parent.Params)
}

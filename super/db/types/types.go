package types

import (
	"context"
	"database/sql"
)

// Map for holding Table columns name and value pointer
type Columns map[string]any

// DBTX is a shared interface for *sql.DB and *sql.Tx,
// allowing driver methods to work with both regular connections and transactions.
type DBTX interface {
	Exec(query string, args ...any) (sql.Result, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

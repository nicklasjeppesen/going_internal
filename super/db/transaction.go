package db

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/nicklasjeppesen/going_internal/super/constants"
	drivers "github.com/nicklasjeppesen/going_internal/super/db/drivers"
	. "github.com/nicklasjeppesen/going_internal/super/db/types"
	"github.com/nicklasjeppesen/going_internal/super/util"
)

// Transactions through the context.
//
// Transaction begins a transaction on the default connection and hands fn a
// context that carries it. Everything that runs with that context joins the
// transaction without being told:
//
//   - every model made with DB(ctx) inside fn (and every relation it loads)
//   - raw SQL through Conn(ctx)
//   - a nested Transaction(ctx, …), which just runs in the outer one
//
//	err := db.Transaction(ctx, func(ctx context.Context) error {
//		if err := todo.DB(ctx).Update(); err != nil {
//			return err // rolls back
//		}
//		return outbox.Store(ctx, todo.ReleaseEvents()...)
//	})
//
// A model made before the transaction began keeps the context it was made
// with, so make the models you write inside fn (or attach the tx with WithTx).

// txKey is the context key of the transaction.
type txKey struct{}

// txState is a transaction together with the pool it was begun on, so a model
// on another database never joins it.
type txState struct {
	tx   *sql.Tx
	pool *sql.DB

	// afterCommit are the functions AfterCommit queued, run once the
	// outermost Transaction has committed. Shared by nested transactions.
	afterCommit *[]func()
}

// Transaction runs fn in a transaction on the default connection (DB_CONNECTION)
// and commits it when fn returns nil. It rolls back when fn returns an error
// (which is returned) or panics (the panic goes on after the rollback).
//
// When ctx already carries a transaction on the same database, fn runs in that
// one: no new transaction is begun, and only the outermost Transaction commits
// or rolls back. An error from the inner fn is returned to the outer one, which
// decides.
func Transaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return transactionOn(ctx, Pool(), fn)
}

// transactionOn is Transaction on pool: the one implementation behind the
// package function and the deprecated ParentDB.Transaction (which uses the
// model's own pool).
func transactionOn(ctx context.Context, pool *sql.DB, fn func(ctx context.Context) error) (err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if state, ok := ctx.Value(txKey{}).(txState); ok && state.pool == pool {
		return fn(ctx)
	}

	tx, err := pool.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db: begin transaction: %w", err)
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			tx.Rollback()
			panic(recovered)
		}
	}()

	var afterCommit []func()
	if err := fn(context.WithValue(ctx, txKey{}, txState{tx: tx, pool: pool, afterCommit: &afterCommit})); err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("db: commit transaction: %w", err)
	}
	for _, f := range afterCommit {
		f()
	}
	return nil
}

// AfterCommit runs f once the transaction in ctx has committed, or right away
// when ctx has no transaction. After a rollback f never runs. Use it for what
// must only happen when the change is stored, such as delivering its events.
//
// f gets no context: capture one made with WithoutTx, since the transaction is
// over when f runs.
func AfterCommit(ctx context.Context, f func()) {
	if state, ok := ctx.Value(txKey{}).(txState); ok && state.tx != nil && state.afterCommit != nil {
		*state.afterCommit = append(*state.afterCommit, f)
		return
	}
	f()
}

// WithoutTx is ctx without its transaction (its other values are kept), for
// work that must not join it, such as a goroutine that outlives it.
func WithoutTx(ctx context.Context) context.Context {
	if TxFrom(ctx) == nil {
		return ctx
	}
	return context.WithValue(ctx, txKey{}, txState{})
}

// TxFrom returns the transaction ctx carries, or nil when there is none.
func TxFrom(ctx context.Context) *sql.Tx {
	if ctx == nil {
		return nil
	}
	if state, ok := ctx.Value(txKey{}).(txState); ok {
		return state.tx
	}
	return nil
}

// Conn is what raw SQL should run on: the transaction ctx carries, or else the
// default pool. Pass the same ctx to ExecContext/QueryContext.
//
//	db.Conn(ctx).ExecContext(ctx, db.Rebind("UPDATE todos SET title = ? WHERE id = ?"), title, id)
func Conn(ctx context.Context) DBTX {
	if tx := TxFrom(ctx); tx != nil {
		return tx
	}
	return Pool()
}

// Pool is the shared connection pool of the default connection (DB_CONNECTION),
// the same one the ORM uses. Use it for raw SQL instead of opening a pool of
// your own: SQLite allows one writer at a time, and two pools on the same file
// make their writers wait for each other. Never Close it.
func Pool() *sql.DB {
	creator := drivers.DefaultDBConnection(context.Background())
	return creator.Driver.Open(creator.ConnectionString)
}

// Rebind turns the ? placeholders of query into the default connection's
// style: unchanged for SQLite, $1, $2, … for Postgres. A ? inside a quoted
// string in the query is not supported.
func Rebind(query string) string {
	if util.GetEnv(constants.DB_CONNECTION, "") != "Postgres" {
		return query
	}

	var out strings.Builder
	n := 0
	for _, r := range query {
		if r == '?' {
			n++
			out.WriteString("$" + strconv.Itoa(n))
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}

// ctxTx is the transaction in ctx when it was begun on pool, so a model only
// joins a transaction on its own database.
func ctxTx(ctx context.Context, pool *sql.DB) *sql.Tx {
	if ctx == nil {
		return nil
	}
	if state, ok := ctx.Value(txKey{}).(txState); ok && state.pool == pool {
		return state.tx
	}
	return nil
}

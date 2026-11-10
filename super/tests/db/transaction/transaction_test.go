package transaction

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"

	"github.com/nicklasjeppesen/going_internal/super/db"
	. "github.com/nicklasjeppesen/going_internal/super/tests/db/shared"
	model "github.com/nicklasjeppesen/going_internal/super/tests/db/shared/models"
	schema "github.com/nicklasjeppesen/going_internal/super/tests/db/shared/schema"
)

// Tests for transactions through the context: db.Transaction, TxFrom, Conn,
// Pool, and models made with DB(ctx) joining the transaction.

var errStop = errors.New("stop")

// setup makes a database with users and points the default connection at the
// same file the shared models use, so db.Pool and the models share one pool.
func setup(t *testing.T) *TestDB {
	t.Helper()
	database, err := NewInMemoryDB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		database.DB.Close()
		for _, f := range []string{"testdb.db", "testdb.db-wal", "testdb.db-shm"} {
			os.Remove(f)
		}
	})
	if err := database.LoadSchema(schema.UserSchema); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DB_CONNECTION", "sqlite")
	t.Setenv("DB_PATH", "./testdb.db")
	return database
}

// users is the number of users called name.
func users(t *testing.T, database *TestDB, name string) int {
	t.Helper()
	var n int
	if err := database.DB.QueryRow("SELECT COUNT(*) FROM users WHERE name = ?", name).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// saveUser saves a user called name through the ORM with ctx.
func saveUser(ctx context.Context, name string) error {
	user := model.User{}.DB(ctx)
	user.Name = name
	_, err := user.Save()
	return err
}

func TestAnORMRowRollsBackWithTheTransaction(t *testing.T) {
	database := setup(t)
	ctx := context.Background()

	err := db.Transaction(ctx, func(ctx context.Context) error {
		if err := saveUser(ctx, "rolled back"); err != nil {
			return err
		}
		// Inside, the model reads its own uncommitted row through the tx.
		if found := (model.User{}).DB(ctx).Where("name", "rolled back").First(); found.IsEmpty() {
			t.Error("a model inside the transaction must see the row saved in it")
		}
		return errStop
	})
	if !errors.Is(err, errStop) {
		t.Fatalf("Transaction returned %v, want fn's error", err)
	}
	if n := users(t, database, "rolled back"); n != 0 {
		t.Fatalf("a rolled back row was stored %d times", n)
	}

	if err := db.Transaction(ctx, func(ctx context.Context) error { return saveUser(ctx, "committed") }); err != nil {
		t.Fatal(err)
	}
	if n := users(t, database, "committed"); n != 1 {
		t.Fatalf("a committed row: found %d", n)
	}
}

func TestAPanicRollsBackAndGoesOn(t *testing.T) {
	database := setup(t)

	func() {
		defer func() {
			if recover() != "boom" {
				t.Fatal("the panic must go on after the rollback")
			}
		}()
		db.Transaction(context.Background(), func(ctx context.Context) error {
			saveUser(ctx, "panicked")
			panic("boom")
		})
	}()

	if n := users(t, database, "panicked"); n != 0 {
		t.Fatalf("a row from a panicking transaction was stored %d times", n)
	}
}

func TestANestedTransactionReusesTheOuterOne(t *testing.T) {
	database := setup(t)
	ctx := context.Background()

	err := db.Transaction(ctx, func(outer context.Context) error {
		if err := db.Transaction(outer, func(inner context.Context) error {
			if db.TxFrom(inner) != db.TxFrom(outer) {
				t.Error("the nested transaction must reuse the outer tx")
			}
			return saveUser(inner, "inner")
		}); err != nil {
			return err
		}
		// The inner one didn't commit: the outer one still decides.
		return errStop
	})
	if !errors.Is(err, errStop) {
		t.Fatal(err)
	}
	if n := users(t, database, "inner"); n != 0 {
		t.Fatalf("the inner row must roll back with the outer transaction, found %d", n)
	}

	// An error from the inner one reaches the outer one.
	err = db.Transaction(ctx, func(outer context.Context) error {
		return db.Transaction(outer, func(context.Context) error { return errStop })
	})
	if !errors.Is(err, errStop) {
		t.Fatalf("the inner error: got %v", err)
	}
}

func TestConnFollowsTheTransaction(t *testing.T) {
	database := setup(t)
	ctx := context.Background()

	if db.TxFrom(ctx) != nil {
		t.Fatal("a plain context has no transaction")
	}
	if pool, ok := db.Conn(ctx).(*sql.DB); !ok || pool != db.Pool() {
		t.Fatal("outside a transaction, Conn is the shared pool")
	}

	err := db.Transaction(ctx, func(ctx context.Context) error {
		if db.Conn(ctx) != db.TxFrom(ctx) || db.TxFrom(ctx) == nil {
			t.Error("inside a transaction, Conn is its tx")
		}
		_, err := db.Conn(ctx).ExecContext(ctx, db.Rebind("INSERT INTO users (name, age) VALUES (?, ?)"), "raw", 1)
		if err != nil {
			return err
		}
		return errStop
	})
	if !errors.Is(err, errStop) {
		t.Fatal(err)
	}
	if n := users(t, database, "raw"); n != 0 {
		t.Fatalf("raw SQL through Conn must roll back with the transaction, found %d", n)
	}
}

func TestTheModelsTransactionJoinsTheContextOne(t *testing.T) {
	database := setup(t)

	err := db.Transaction(context.Background(), func(ctx context.Context) error {
		user := model.User{}.DB(ctx)
		return user.Transaction(func(tx *sql.Tx) error {
			if tx != db.TxFrom(ctx) {
				t.Error("ParentDB.Transaction inside db.Transaction must use its tx")
			}
			user.Name = "model tx"
			if _, err := user.Save(); err != nil {
				return err
			}
			return nil
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	if n := users(t, database, "model tx"); n != 1 {
		t.Fatalf("found %d", n)
	}
}

func TestRebind(t *testing.T) {
	t.Setenv("DB_CONNECTION", "sqlite")
	if got := db.Rebind("a = ? AND b = ?"); got != "a = ? AND b = ?" {
		t.Fatalf("sqlite: %q", got)
	}
	t.Setenv("DB_CONNECTION", "Postgres")
	if got := db.Rebind("a = ? AND b = ?"); got != "a = $1 AND b = $2" {
		t.Fatalf("postgres: %q", got)
	}
}

func TestTheModelsTransactionCommitsAndRollsBack(t *testing.T) {
	database := setup(t)

	// Commit: fn returns nil.
	user := model.User{}.DB(context.Background())
	err := user.Transaction(func(tx *sql.Tx) error {
		if tx == nil || user.DbConn() != tx {
			t.Error("the model's connection must be the tx while fn runs")
		}
		user.Name = "committed"
		_, err := user.Save()
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if n := users(t, database, "committed"); n != 1 {
		t.Fatalf("commit: found %d", n)
	}
	if _, isTx := user.DbConn().(*sql.Tx); isTx {
		t.Error("the model's connection must be restored after Transaction")
	}

	// Rollback: fn returns an error.
	user = model.User{}.DB(context.Background())
	err = user.Transaction(func(tx *sql.Tx) error {
		user.Name = "rolled back"
		if _, err := user.Save(); err != nil {
			return err
		}
		return errStop
	})
	if !errors.Is(err, errStop) {
		t.Fatal(err)
	}
	if n := users(t, database, "rolled back"); n != 0 {
		t.Fatalf("rollback: found %d", n)
	}

	// Panic: rolled back, and the panic goes on.
	user = model.User{}.DB(context.Background())
	func() {
		defer func() {
			if recover() == nil {
				t.Error("the panic must go on after the rollback")
			}
		}()
		user.Transaction(func(tx *sql.Tx) error {
			user.Name = "panicked"
			user.Save()
			panic("boom")
		})
	}()
	if n := users(t, database, "panicked"); n != 0 {
		t.Fatalf("panic: found %d", n)
	}
}

func TestTheModelsTransactionLeavesTheOuterOneToDecide(t *testing.T) {
	database := setup(t)

	// The inner (model) Transaction succeeds, but the outer one rolls back:
	// nothing is stored, since only the outermost commits.
	err := db.Transaction(context.Background(), func(ctx context.Context) error {
		user := model.User{}.DB(ctx)
		if err := user.Transaction(func(tx *sql.Tx) error {
			user.Name = "inner"
			_, err := user.Save()
			return err
		}); err != nil {
			return err
		}
		return errStop
	})
	if !errors.Is(err, errStop) {
		t.Fatal(err)
	}
	if n := users(t, database, "inner"); n != 0 {
		t.Fatalf("the outer rollback must undo the inner one, found %d", n)
	}
}

package pool

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/nicklasjeppesen/going_internal/super/db/drivers"
)

// newDB starts from an empty database file and returns a driver factory.
func newDB(t *testing.T, path string) func() *drivers.SQLite {
	t.Helper()
	os.Remove(path)
	drivers.ResetPools()
	t.Cleanup(func() {
		drivers.ResetPools()
		os.Remove(path)
		os.Remove(path + "-wal")
		os.Remove(path + "-shm")
	})
	driver := func() *drivers.SQLite {
		return drivers.CreateSQLiteCustom(context.Background(), path).Driver.(*drivers.SQLite)
	}
	return driver
}

// SQLite runs in WAL mode by default.
func TestSQLiteUsesWAL(t *testing.T) {
	path := "./waltest.db"
	driver := newDB(t, path)
	var mode string
	if err := driver().Open(path).QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", mode)
	}
}

// A failing query returns an error (or an empty result) instead of calling
// log.Fatal, which used to stop the whole server.
func TestFailingQueriesDoNotExit(t *testing.T) {
	path := "./errtest.db"
	driver := newDB(t, path)
	db := driver().Open(path)

	d := driver()
	d.SetTable("missing_table")
	if _, err := d.Save_(db, []string{"name"}, []any{"x"}, nil); err == nil {
		t.Error("Save_ into a missing table: want error")
	}
	d = driver()
	d.SetTable("missing_table")
	if _, err := d.Save_(db, []string{"name"}, []any{"x"}, []string{"id"}); err == nil {
		t.Error("Save_ with RETURNING into a missing table: want error")
	}
	d = driver()
	d.SetTable("missing_table")
	if err := d.Update_(db, []string{"name"}, []any{"x"}); err == nil {
		t.Error("Update_ on a missing table: want error")
	}
	d = driver()
	d.SetTable("missing_table")
	if err := d.Delete_(db, 1); err == nil {
		t.Error("Delete_ on a missing table: want error")
	}
	d = driver()
	d.SetTable("missing_table")
	if rows := d.Get_(db, []string{"name"}); rows != nil {
		t.Errorf("Get_ on a missing table = %v, want nil", rows)
	}
}

// Many concurrent writers must not fail with "database is locked".
func TestConcurrentWritesDoNotLock(t *testing.T) {
	path := "./locktest.db"
	driver := newDB(t, path)
	db := driver().Open(path)
	if _, err := db.Exec("CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 1000)
	for w := 0; w < 20; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 25; i++ {
				d := driver()
				d.SetTable("items")
				if _, err := d.Save_(db, []string{"name"}, []any{fmt.Sprint(w, "-", i)}, []string{"id"}); err != nil {
					errs <- err
				}
				d = driver()
				d.SetTable("items")
				d.Get_(db, []string{"id", "name"})
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	var n int
	db.QueryRow("SELECT count(*) FROM items").Scan(&n)
	if n != 500 {
		t.Fatalf("rows = %d, want 500", n)
	}
}

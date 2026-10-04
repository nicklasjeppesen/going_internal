package pool

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/nicklasjeppesen/going_internal/super/db/drivers"
)

// The ORM must reuse one connection pool per database instead of opening
// (and leaking) a new one per query.
func TestOpenReturnsSharedPool(t *testing.T) {
	path := "./pooltest.db"
	t.Cleanup(func() { drivers.ResetPools(); os.Remove(path) })
	creator := drivers.CreateSQLiteCustom(context.Background(), path)

	first := creator.Driver.Open(path)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got := creator.Driver.Open(path); got != first {
				t.Error("Open returned a different pool")
			}
		}()
	}
	wg.Wait()

	if err := first.Ping(); err != nil {
		t.Fatalf("shared pool not usable: %v", err)
	}

	// SQLite: a bounded pool, so concurrent writers queue in Go instead of
	// sleeping in SQLite's busy handler.
	if n := first.Stats().MaxOpenConnections; n < 16 {
		t.Errorf("MaxOpenConnections = %d, want at least 16", n)
	}

	drivers.ResetPools()
	if creator.Driver.Open(path) == first {
		t.Fatal("ResetPools did not replace the pool")
	}
}

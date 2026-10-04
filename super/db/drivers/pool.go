package drivers

import (
	"database/sql"
	"fmt"
	"runtime"
	"sync"
	"time"
)

// pools holds one *sql.DB per driver + connection string. A *sql.DB is a
// connection pool meant to be long-lived and shared: opening one per query
// means a new database connection (for Postgres a TCP handshake and login)
// for every query, and any pool that isn't closed leaks its connections.
var pools sync.Map // key: driverName + "\x00" + connectionString → *sql.DB

// sharedPool returns the process-wide pool for the connection string,
// creating it on first use. The returned *sql.DB must NOT be closed.
func sharedPool(driverName, connectionString string) *sql.DB {
	key := driverName + "\x00" + connectionString
	if db, ok := pools.Load(key); ok {
		return db.(*sql.DB)
	}

	db, err := sql.Open(driverName, connectionString)
	if err != nil {
		// sql.Open only fails for an unregistered driver name (a programming
		// error); panic so the request fails instead of the whole process.
		panic(fmt.Sprintf("open %s database: %v", driverName, err))
	}
	db.SetConnMaxIdleTime(10 * time.Minute)
	// Keep a few connections ready for concurrent requests instead of
	// opening and closing one each time (the default keeps only 2).
	db.SetMaxIdleConns(8)

	if driverName == "sqlite3" {
		// SQLite allows one writer at a time. With an unlimited pool, hundreds
		// of concurrent writers wait inside SQLite's busy handler, which sleeps
		// in steps and hands the lock over slowly: under load the throughput
		// collapsed (about 1 message/s instead of about 290/s in a load test on
		// one core). A cap keeps the queue in Go, where a freed connection is
		// handed over at once. Queries inside Transaction() must use the tx
		// (WithTx/SetDbConn), so they never wait for a pooled connection while
		// the transaction holds one.
		maxConns := max(16, 4*runtime.GOMAXPROCS(0))
		db.SetMaxOpenConns(maxConns)
		db.SetMaxIdleConns(maxConns)
	}

	if existing, loaded := pools.LoadOrStore(key, db); loaded {
		db.Close() // another goroutine won the race; use its pool
		return existing.(*sql.DB)
	}
	return db
}

// ResetPools closes every shared pool; the next query opens a fresh one.
// Only needed when a database file is deleted or replaced while the process
// runs (e.g. tests that recreate their database) — open connections would
// otherwise keep using the old file.
func ResetPools() {
	pools.Range(func(key, db any) bool {
		db.(*sql.DB).Close()
		pools.Delete(key)
		return true
	})
}

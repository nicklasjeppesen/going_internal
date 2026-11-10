package outbox

import (
	_ "embed"
	"fmt"
)

// The outbox's two tables, domain_events and event_deliveries. Add them to the
// app's migrations (copy the SQL into a migration file), or run Schema's SQL
// once at startup; every statement is CREATE … IF NOT EXISTS.

// SQLiteSchema is the outbox's tables for SQLite.
//
//go:embed schema/sqlite.sql
var SQLiteSchema string

// PostgresSchema is the outbox's tables for Postgres.
//
//go:embed schema/postgres.sql
var PostgresSchema string

// Schema is the outbox's tables for driver, named as in DB_CONNECTION
// ("sqlite" or "Postgres").
func Schema(driver string) (string, error) {
	switch driver {
	case "sqlite":
		return SQLiteSchema, nil
	case "Postgres":
		return PostgresSchema, nil
	}
	return "", fmt.Errorf("outbox: no schema for the driver %q", driver)
}

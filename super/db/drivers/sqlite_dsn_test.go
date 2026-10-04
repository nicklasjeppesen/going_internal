package drivers

import "testing"

func TestSqliteDSN(t *testing.T) {
	cases := map[string]string{
		"./data/app.db":              "./data/app.db?_journal_mode=WAL&_busy_timeout=5000&_txlock=immediate",
		"file:./app.db?cache=shared": "file:./app.db?cache=shared&_journal_mode=WAL&_busy_timeout=5000&_txlock=immediate",
		// Settings in the connection string win, also under the driver's aliases
		"./app.db?_journal=DELETE&_timeout=100&_txlock=deferred": "./app.db?_journal=DELETE&_timeout=100&_txlock=deferred",
		"./app.db?_journal_mode=TRUNCATE":                        "./app.db?_journal_mode=TRUNCATE&_busy_timeout=5000&_txlock=immediate",
		// In-memory databases are left alone
		":memory:":                           ":memory:",
		"file:test?mode=memory&cache=shared": "file:test?mode=memory&cache=shared",
		"":                                   "",
	}
	for in, want := range cases {
		if got := sqliteDSN(in); got != want {
			t.Errorf("sqliteDSN(%q) = %q, want %q", in, got, want)
		}
	}
}

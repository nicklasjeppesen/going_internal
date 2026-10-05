package drivers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/nicklasjeppesen/going_internal/super/constants"
	types "github.com/nicklasjeppesen/going_internal/super/db/types"
	"github.com/nicklasjeppesen/going_internal/super/util"

	_ "github.com/mattn/go-sqlite3"
)

// SQLite3 driver see for help - https://www.sqlitetutorial.net/sqlite-go/

type SQLite struct {
	Conditions    []string
	Params        []any
	IsWhere       bool
	table         string
	withLimit     bool
	limit         int
	shouldOrderBy bool     // Determine if the records shall be order by a column
	orderBy       []string // What column shall be order
	offSet        int
	withOffSet    bool
	lockClause    string
	ctx           context.Context
}

func CreateSQLite(ctx context.Context) types.DBCreator {
	var dbpath = util.GetEnv(constants.DB_PATH, "")
	return types.DBCreator{
		Driver:           &SQLite{ctx: ctx},
		ConnectionString: dbpath,
	}
}

func CreateSQLiteCustom(ctx context.Context, dbpath string) types.DBCreator {
	return types.DBCreator{
		Driver:           &SQLite{ctx: ctx},
		ConnectionString: dbpath,
	}
}

func (parent *SQLite) Clone() types.IDrivers {
	return &SQLite{ctx: parent.ctx}
}

// Open returns the shared connection pool for the database. Do not Close it.
func (parent *SQLite) Open(connectionString string) *sql.DB {
	return sharedPool("sqlite3", sqliteDSN(connectionString))
}

// sqliteDefaults are added to the connection string unless it sets them
// itself (also under the driver's alias, e.g. _journal for _journal_mode):
//   - WAL lets readers work while one writer writes, instead of failing with
//     "database is locked" under concurrent requests (Rails 8 does the same).
//     SQLite keeps -wal and -shm files next to the database in this mode.
//   - busy_timeout makes a writer wait up to 5 seconds for the write lock.
//   - txlock=immediate takes the write lock when a transaction begins, so two
//     transactions can't deadlock while upgrading from reading to writing.
var sqliteDefaults = []struct {
	key, alias, value string
}{
	{"_journal_mode", "_journal", "WAL"},
	{"_busy_timeout", "_timeout", "5000"},
	{"_txlock", "", "immediate"},
}

// sqliteDSN adds sqliteDefaults to a connection string. In-memory databases
// are left as they are.
func sqliteDSN(connectionString string) string {
	if connectionString == "" || strings.Contains(connectionString, ":memory:") || strings.Contains(connectionString, "mode=memory") {
		return connectionString
	}

	var params url.Values
	if i := strings.IndexByte(connectionString, '?'); i >= 0 {
		params, _ = url.ParseQuery(connectionString[i+1:])
	}

	var add []string
	for _, d := range sqliteDefaults {
		if params.Has(d.key) || (d.alias != "" && params.Has(d.alias)) {
			continue
		}
		add = append(add, d.key+"="+d.value)
	}
	if len(add) == 0 {
		return connectionString
	}

	separator := "?"
	if strings.Contains(connectionString, "?") {
		separator = "&"
	}
	return connectionString + separator + strings.Join(add, "&")
}

// Actions

func (parent *SQLite) Get_(_db types.DBTX, columns []string) [][]any {

	var query = parent.querySelectMaker(columns)

	rows, err := _db.QueryContext(parent.ctx, query, parent.Params...)
	if err != nil {
		log.Printf("SQLite query failed: %v (%s)", err, query)
		return nil
	}
	defer rows.Close()

	var mylist [][]any

	for rows.Next() {

		values := make([]any, len(columns))
		for i := range values {
			values[i] = new(any) // create addressable placeholder
		}

		if err := rows.Scan(values[:]...); err != nil {
			log.Printf("SQLite scan failed: %v (%s)", err, query)
			return nil
		}
		mylist = append(mylist, values)
	}

	// Check for error during iteration
	if err := rows.Err(); err != nil {
		log.Printf("SQLite rows failed: %v (%s)", err, query)
		return nil
	}

	return mylist

}

func (parent *SQLite) Save_(_db types.DBTX, columns []string, values []any, returningValues []string) ([]any, error) {

	var placeholders = make([]string, len(values))
	for i := range values {
		placeholders[i] = "?"
	}

	returning := returning(returningValues)
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) %s",
		parent.table,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
		returning,
	)

	if len(returningValues) == 0 {
		// Without RETURNING, use Exec instead of QueryRow
		if _, err := _db.ExecContext(parent.ctx, query, values...); err != nil {
			return nil, fmt.Errorf("insert into %s: %w", parent.table, err)
		}
		return nil, nil
	}

	result := make([]any, len(returningValues))
	scanArgs := make([]any, len(returningValues))
	for i := range result {
		scanArgs[i] = &result[i]
	}

	if err := _db.QueryRowContext(parent.ctx, query, values...).Scan(scanArgs...); err != nil {
		return nil, fmt.Errorf("insert into %s: %w", parent.table, err)
	}
	return result, nil

}

func returning(returningValues []string) string {
	if len(returningValues) == 0 {
		return ""
	}
	return "RETURNING " + strings.Join(returningValues, ",")
}

// first

func (parent *SQLite) First_(_db types.DBTX, columns []string) []any {

	// Run SELECT query
	var query = parent.querySelectMaker(columns)
	query += " LIMIT 1"

	row := _db.QueryRowContext(parent.ctx, query, parent.Params...)
	values := make([]any, len(columns))

	for i := range values {
		values[i] = new(any) // create addressable placeholder
	}

	err := row.Scan(values[:]...)

	if err != nil {
		// No rows is a normal result (First returns the empty model); only
		// log real errors.
		if !errors.Is(err, sql.ErrNoRows) {
			fmt.Println(err.Error())
		}
		return nil
	}
	// Check for error during iteration
	if err := row.Err(); err != nil {
		//log.Fatal(err)
		fmt.Println(err)
		return nil
	}

	return values
}

// Update
func (parent *SQLite) Update_(_db types.DBTX, columns []string, values []any) error {

	table := parent.table
	var placeholders = make([]string, len(values))

	var paramLenght int = len(parent.Params)
	for index, v := range columns {
		placeholders[index] = v + " = ?" + strconv.Itoa(paramLenght+index+1)
	}

	query := fmt.Sprintf("UPDATE %s SET %s ",
		table, strings.Join(placeholders, ", "))

	// Combine the values
	var accumaltedValues = append(parent.Params, values...)

	query = parent.queryUpdateMaker(query)
	if _, err := _db.ExecContext(parent.ctx, query, accumaltedValues...); err != nil {
		return fmt.Errorf("update %s: %w", table, err)
	}
	return nil
}

// Delete
func (parent *SQLite) Delete_(_db types.DBTX, id any) error {

	if id != nil {
		parent.Where_("id", []any{id}) // Add the ID
	}
	var query = parent.queryDeleteMaker()

	if _, err := _db.ExecContext(parent.ctx, query, parent.Params...); err != nil {
		return fmt.Errorf("delete from %s: %w", parent.table, err)
	}

	return nil
}

func (parent *SQLite) querySelectMaker(columns []string) string {

	keystring := strings.Join(columns, ",")
	query := fmt.Sprintf("SELECT %s FROM %s", keystring, parent.table)

	if len(parent.Conditions) > 0 {
		query += strings.Join(parent.Conditions, " ")
	}

	if parent.shouldOrderBy {
		query += " Order BY " + strings.Join(parent.orderBy, ", ")
	}

	if parent.withLimit {
		query += " Limit " + strconv.Itoa(parent.limit)
	}

	if parent.withOffSet {
		query += " OFFSET " + strconv.Itoa(parent.offSet)
	}

	if parent.lockClause != "" {
		query += " " + parent.lockClause
	}

	return query
}

func (parent *SQLite) queryDeleteMaker() string {

	query := fmt.Sprintf("DELETE FROM %s", parent.table)

	if len(parent.Conditions) > 0 {
		query += strings.Join(parent.Conditions, " ")
	}
	return query

}

func (parent *SQLite) queryUpdateMaker(query string) string {

	if len(parent.Conditions) > 0 {
		query += strings.Join(parent.Conditions, " ")
	}
	return query

}

func (parent *SQLite) SetTable(table string) {
	parent.table = table
}

func (parent *SQLite) whereClause() string {

	var clause string
	if !parent.IsWhere {
		clause = "WHERE"
		parent.IsWhere = true
	} else {
		clause = "AND"

	}
	return clause
}

func (parent *SQLite) Where_(column string, input []any) types.IDrivers {

	var paramNr = "?" + strconv.Itoa(len(parent.Params)+1)
	var operator string
	var value any
	if len(input) == 1 {
		operator = " = "
		value = input[0]
	} else {
		operator = input[0].(string)
		value = input[1]
	}

	var stringQuery = fmt.Sprintf(" %s %s %s "+paramNr, parent.whereClause(), column, operator)
	parent.Conditions = append(parent.Conditions, stringQuery) // Postgress require $, mysql ?
	parent.Params = append(parent.Params, value)
	return parent
}

func (parent *SQLite) Or_(column string, value any) types.IDrivers {

	var paramNr = "?" + strconv.Itoa(len(parent.Params)+1)
	parent.Conditions = append(parent.Conditions, fmt.Sprintf("OR %s = "+paramNr, column))
	parent.Params = append(parent.Params, value)
	return parent
}

func (parent *SQLite) WhereIn_(column string, values []any) types.IDrivers {
	if len(values) == 0 {
		// Avoid invalid SQL like "IN ()"
		parent.Conditions = append(parent.Conditions, "FALSE")
		return parent
	}

	placeholders := make([]string, len(values))
	for i := range values {
		paramIndex := len(parent.Params) + 1 // PostgreSQL params start at $1
		placeholders[i] = fmt.Sprintf("?%d", paramIndex)
		parent.Params = append(parent.Params, values[i])
	}

	condition := fmt.Sprintf(" %s %s IN (%s)", parent.whereClause(), column, strings.Join(placeholders, ", "))
	parent.Conditions = append(parent.Conditions, condition)
	return parent
}

func (parent *SQLite) OrderByDesc_(column string) {

	parent.shouldOrderBy = true
	parent.orderBy = append(parent.orderBy, column+" DESC")

}

func (parent *SQLite) OrderBy_(column string) {

	parent.shouldOrderBy = true
	parent.orderBy = append(parent.orderBy, column+" ASC")

}

func (parent *SQLite) LockForUpdate_() {
	log.Println("WARNING: LockForUpdate() is not supported by SQLite and has no effect.")
}

func (parent *SQLite) SharedLock_() {
	log.Println("WARNING: SharedLock() is not supported by SQLite and has no effect.")
}

func (parent *SQLite) Limit_(max int) {
	parent.withLimit = true
	parent.limit = max
}

func (parent *SQLite) OffSet_(max int) {
	parent.withOffSet = true
	parent.offSet = max
}

func (parent *SQLite) CreateMigrationTable() string {
	return `
		CREATE TABLE IF NOT EXISTS migrations (
			id INTEGER PRIMARY key,
			filename TEXT UNIQUE NOT NULL,
			applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`
}

func (parent *SQLite) Migrate(scriptpath string) error {

	// 1. Get DB
	var dbpath = util.GetEnv(constants.DB_PATH, "")
	db, err := sql.Open("sqlite3", dbpath)
	if err != nil {
		log.Fatalf("Error trying opening the database: %v", err)
	}

	defer db.Close()

	// 2. Make sure the migration table do exists in the DB
	_, err = db.Exec(parent.CreateMigrationTable())
	if err != nil {
		log.Fatalf("Error connection to migration tabel: %v", err)
	}

	// 3. migrate the new files.
	err = parent.LoadMigrationFile(scriptpath, db)
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	fmt.Println("All Migration executed successfully")
	return nil

}

func (parent *SQLite) LoadMigrationFile(basePath string, db *sql.DB) error {

	files, err := os.ReadDir(basePath)
	if err != nil {
		log.Fatalf("Could not read the folder: %v", err)
	}

	// Filtering only *.SQL
	var migrations []string
	for _, f := range files {
		if !f.IsDir() && strings.ToLower(filepath.Ext(f.Name())) == ".sql" {
			migrations = append(migrations, f.Name())
		}
	}

	// Sorting in ASC order
	sort.Strings(migrations)

	// Running each migration
	for _, m := range migrations {

		// Check if the file already exists in the migration table
		var migrationAlreadyRun bool
		err := db.QueryRow("SELECT EXISTS (SELECT 1 FROM migrations WHERE filename = $1)", m).Scan(&migrationAlreadyRun)
		if err != nil {
			log.Fatalf("Error by checking migration tabel: %v", err)
		}

		if migrationAlreadyRun {
			continue
		}

		fmt.Printf("Running migration: %s\n", m)

		fullPath := filepath.Join(basePath, m)
		sqlBytes, err := os.ReadFile(fullPath)
		if err != nil {
			log.Fatalf("Could not read %s: %v", m, err)
		}

		_, err = db.ExecContext(parent.ctx, string(sqlBytes))
		if err != nil {
			return fmt.Errorf("error executing SQL in %s: %w", m, err)
		}

		// insert into the migration table
		_, err = db.Exec("INSERT INTO migrations (filename) VALUES ($1)", m)
		if err != nil {
			return fmt.Errorf("error inserting %s into migrations table: %w", m, err)
		}
	}

	return nil

}

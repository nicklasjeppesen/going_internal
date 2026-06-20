package drivers

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
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
	ctx           context.Context
}

func CreateSQLite(ctx context.Context) types.DBCreator {
	var dbpath = util.GetEnv(constants.DB_PATH, "")
	return types.DBCreator{
		Driver:           &SQLite{ctx: ctx},
		ConnectionString: dbpath,
	}
}

func (parent *SQLite) Clone() types.IDrivers {
	return &SQLite{ctx: parent.ctx}
}

func (parent *SQLite) Open(connectionString string) *sql.DB {

	_db, err := sql.Open("sqlite3", connectionString)
	if err != nil {
		log.Fatal(err)
	}
	return _db // remember to close DB when getting it.
}

// Actions

func (parent *SQLite) Get_(_db *sql.DB, columns []string) [][]any {

	defer _db.Close()
	var query = parent.querySelectMaker(columns)

	rows, err := _db.QueryContext(parent.ctx, query, parent.Params...)
	if err != nil {
		log.Fatal(err)
		fmt.Println("Error getting rows")
	}
	defer rows.Close()

	var mylist [][]any

	for rows.Next() {

		values := make([]any, len(columns))
		for i := range values {
			values[i] = new(any) // create addressable placeholder
		}

		err := rows.Scan(values[:]...)
		if err != nil {
			log.Fatal(err)
			fmt.Println("Error getting rows2")
		}
		mylist = append(mylist, values)
	}

	// Check for error during iteration
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}

	return mylist

}

func (parent *SQLite) Save_(_db *sql.DB, columns []string, values []any, returningValues []string) []any {

	defer _db.Close()

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
		// Hvis ingen RETURNING, brug Exec i stedet for QueryRow
		_, err := _db.ExecContext(parent.ctx, query, values...)
		if err != nil {
			log.Fatal(err)
		}
		return nil
	}

	result := make([]any, len(returningValues))
	scanArgs := make([]any, len(returningValues))
	for i := range result {
		scanArgs[i] = &result[i]
	}

	err := _db.QueryRow(query, values...).Scan(scanArgs...)
	if err != nil {
		log.Fatal(err)
	}
	return result

}

func returning(returningValues []string) string {
	if len(returningValues) == 0 {
		return ""
	}
	return "RETURNING " + strings.Join(returningValues, ",")
}

// first

func (parent *SQLite) First_(_db *sql.DB, columns []string) []any {

	defer _db.Close()

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
		fmt.Println(err.Error())
		// Meaning, no rows found
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
func (parent *SQLite) Update_(_db *sql.DB, columns []string, values []any) {

	table := parent.table
	var placeholders = make([]string, len(values))

	var paramLenght int = len(parent.Params)
	for index, v := range columns {
		placeholders[index] = v + " = ?" + strconv.Itoa(paramLenght+index+1)
	}

	query := fmt.Sprintf("UPDATE %s SET %s ",
		table, strings.Join(placeholders, ", "))
	defer _db.Close()

	// Combine the values
	var accumaltedValues = append(parent.Params, values...)

	query = parent.queryUpdateMaker(query)
	_, err := _db.Exec(query, accumaltedValues...)
	if err != nil {
		log.Fatal(err.Error())
	}
}

// Delete
func (parent *SQLite) Delete_(_db *sql.DB, id any) error {

	defer _db.Close()
	if id != nil {
		parent.Where_("id", []any{id}) // Add the ID
	}
	var query = parent.queryDeleteMaker()

	_, err := _db.ExecContext(parent.ctx, query, parent.Params...)
	if err != nil {
		log.Fatal(err.Error())
		return err
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
	// 2. Verfify Migration tabel exists
	// 3. Add migration file to the DB

	util.LoadEnv() // Load the environment variable
	var dbpath = util.GetEnv(constants.DB_PATH, "")
	db, err := sql.Open("sqlite3", dbpath)
	if err != nil {
		log.Fatalf("Error trying opening the database: %v", err)
	}

	defer db.Close()

	// Sikr at migrations tabellen findes
	_, err = db.Exec(parent.CreateMigrationTable())
	if err != nil {
		log.Fatalf("Error connection to migration tabel: %v", err)
	}

	// Reading all files in ./scripts folder
	files, err := os.ReadDir(scriptpath)
	if err != nil {
		log.Fatalf("Could not read the folder: %v", err)
	}

	// Filtering only *.SQL
	var migrations []string
	for _, f := range files {
		if !f.IsDir() && filepathExt(f.Name()) == ".sql" {
			migrations = append(migrations, f.Name())
		}
	}

	// Sorting in ASC order
	sort.Strings(migrations)

	// Running each migration
	for _, m := range migrations {

		// Check if the file already exists in the migration tabel
		var migrationAlreadyRun bool
		err := db.QueryRow("SELECT EXISTS (SELECT 1 FROM migrations WHERE filename = $1)", m).Scan(&migrationAlreadyRun)
		if err != nil {
			log.Fatalf("Error by checking migration tabel: %v", err)
		}

		if migrationAlreadyRun {
			continue
		}

		fmt.Printf("Running migration: %s\n", m)

		sqlBytes, err := os.ReadFile(scriptpath + m)
		if err != nil {
			log.Fatalf("Could not read %s: %v", m, err)
		}

		_, err = db.Exec(string(sqlBytes))
		if err != nil {
			fmt.Printf("Error by running %s: %v", m, err)
		}

		// inser into the migration tabel
		_, err = db.Exec("INSERT INTO migrations (filename) VALUES ($1)", m)
		if err != nil {
			log.Fatalf("Error trying insert migration into migration tabel: %v", err)
		}

	}

	fmt.Println("All Migration executed successfully")
	return nil

}

func filepathExt(path string) string {
	if len(path) < 4 {
		return ""
	}
	return path[len(path)-4:]
}

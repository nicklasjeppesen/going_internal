package drivers

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/nicklasjeppesen/going_internal/super/constants"
	types "github.com/nicklasjeppesen/going_internal/super/db/types"
	"github.com/nicklasjeppesen/going_internal/super/util"
)

// Postgress driver

type PostgresDB struct {
	Conditions       []string
	Params           []any
	IsWhere          bool
	table            string
	withLimit        bool
	limit            int
	shouldOrderBy    bool     // Determine if the records shall be order by a column
	orderBy          []string // What column shall be
	offSet           int
	withOffSet       bool
	ctx              context.Context
	connectionString string
}

func CreatePostgressDB(ctx context.Context) types.DBCreator {
	var host = util.GetEnv(constants.DB_HOST, "")
	var user = util.GetEnv(constants.DB_USER, "")
	var password = util.GetEnv(constants.DB_PASS, "")
	var dbname = util.GetEnv(constants.DB_DATABASE, "")
	ConnectionString := fmt.Sprintf("host=%s user=%s password=%s dbname=%s sslmode=disable", host, user, password, dbname)
	return types.DBCreator{
		Driver:           &PostgresDB{ctx: ctx, connectionString: ConnectionString},
		ConnectionString: ConnectionString,
	}
}

func (parent *PostgresDB) Clone() types.IDrivers {
	return &PostgresDB{ctx: parent.ctx}
}

func (parent *PostgresDB) Open(connectionString string) *sql.DB {

	_db, err := sql.Open("postgres", connectionString)
	if err != nil {
		log.Fatal(err)
	}
	return _db // remember to close DB when getting it.
}

// Actions

func (parent *PostgresDB) Get_(_db *sql.DB, columns []string) [][]any {

	defer _db.Close()
	var query = parent.querySelectMaker(columns)

	rows, err := _db.Query(query, parent.Params...)
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
			fmt.Println("Error getting rows")
		}
		mylist = append(mylist, values)
	}

	// Check for error during iteration
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}

	return mylist

}

func (parent *PostgresDB) Save_(_db *sql.DB, columns []string, values []any, returningValues []string) []any {

	table := parent.table
	defer _db.Close()

	var placeholders = make([]string, len(values))

	for index, _ := range values {
		placeholders[index] = "$" + strconv.Itoa(index+1)
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) "+strings.Join(returningValues, ", "),
		table, strings.Join(columns, ", "), strings.Join(placeholders, ", "))

	result := make([]any, len(returningValues))
	scanArgs := make([]any, len(returningValues))
	for i := range result {
		scanArgs[i] = &result[i]
	}

	err := _db.QueryRowContext(parent.ctx, query, values...).Scan(scanArgs...)
	if err != nil {
		log.Fatal(err)
	}

	if err != nil {
		log.Fatal(err)
	}
	return result

}

// first

func (parent *PostgresDB) First_(_db *sql.DB, columns []string) []any {

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
		fmt.Println(err)
		return nil

	}
	// Check for error during iteration
	if err := row.Err(); err != nil {
		fmt.Println(err)
		return nil
	}
	fmt.Println(values[0] == nil)
	return values
}

// Update
func (parent *PostgresDB) Update_(_db *sql.DB, columns []string, values []any) {

	table := parent.table
	var placeholders = make([]string, len(values))

	var paramLenght int = len(parent.Params)
	for index, v := range columns {
		placeholders[index] = v + " = $" + strconv.Itoa(paramLenght+index+1)
	}

	query := fmt.Sprintf("UPDATE %s SET %s ",
		table, strings.Join(placeholders, ", "))
	defer _db.Close()

	// Combine the values
	var accumaltedValues = append(parent.Params, values...)

	query = parent.queryUpdateMaker(query)
	err := _db.QueryRowContext(parent.ctx, query, accumaltedValues...)
	if err.Err() != nil {
		log.Fatal(err.Err().Error())
	}
}

// Delete
func (parent *PostgresDB) Delete_(_db *sql.DB, id any) error {

	defer _db.Close()
	if id != nil {
		parent.Where_("id", []any{id}) // Add the ID
	}
	var query = parent.queryDeleteMaker()
	err := _db.QueryRowContext(parent.ctx, query, parent.Params...)
	if err.Err() != nil {
		log.Fatal(err.Err().Error())
		return err.Err()
	}

	return nil
}

func (parent *PostgresDB) querySelectMaker(columns []string) string {

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

func (parent *PostgresDB) queryDeleteMaker() string {

	query := fmt.Sprintf("DELETE FROM %s", parent.table)

	if len(parent.Conditions) > 0 {
		query += strings.Join(parent.Conditions, " ")
	}
	return query

}

func (parent *PostgresDB) queryUpdateMaker(query string) string {

	if len(parent.Conditions) > 0 {
		query += strings.Join(parent.Conditions, " ")
	}
	return query

}

func (parent *PostgresDB) SetTable(table string) {
	parent.table = table
}

func (parent *PostgresDB) whereClause() string {

	var clause string
	if !parent.IsWhere {
		clause = "WHERE"
		parent.IsWhere = true
	} else {
		clause = "AND"

	}
	return clause
}

func (parent *PostgresDB) Where_(column string, input []any) types.IDrivers {

	var paramNr = "$" + strconv.Itoa(len(parent.Params)+1)
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

func (parent *PostgresDB) Or_(column string, value any) types.IDrivers {

	var paramNr = "$" + strconv.Itoa(len(parent.Params)+1)
	parent.Conditions = append(parent.Conditions, fmt.Sprintf("OR %s = "+paramNr, column))
	parent.Params = append(parent.Params, value)
	return parent
}

func (parent *PostgresDB) WhereIn_(column string, values []any) types.IDrivers {
	if len(values) == 0 {
		// Avoid invalid SQL like "IN ()"
		parent.Conditions = append(parent.Conditions, "FALSE")
		return parent
	}

	placeholders := make([]string, len(values))
	for i := range values {
		paramIndex := len(parent.Params) + 1 // PostgreSQL params start at $1
		placeholders[i] = fmt.Sprintf("$%d", paramIndex)
		parent.Params = append(parent.Params, values[i])
	}

	condition := fmt.Sprintf(" %s %s IN (%s)", parent.whereClause(), column, strings.Join(placeholders, ", "))
	parent.Conditions = append(parent.Conditions, condition)
	return parent
}

func (parent *PostgresDB) OrderByDesc_(column string) {

	parent.shouldOrderBy = true
	parent.orderBy = append(parent.orderBy, column+" DESC")

}

func (parent *PostgresDB) OrderBy_(column string) {

	parent.shouldOrderBy = true
	parent.orderBy = append(parent.orderBy, column+" ASC")

}

func (parent *PostgresDB) Limit_(max int) {
	parent.withLimit = true
	parent.limit = max
}

func (parent *PostgresDB) OffSet_(max int) {
	parent.withOffSet = true
	parent.offSet = max
}

func (parent *PostgresDB) CreateMigrationTable() string {
	return `
		CREATE TABLE IF NOT EXISTS migrations (
			id SERIAL PRIMARY KEY,
			filename TEXT UNIQUE NOT NULL,
			applied_at TIMESTAMP NOT NULL DEFAULT now()
		);
	`
}

// TODO implement
func (parent *PostgresDB) Migrate(scriptpath string) error {

	db := parent.Open(parent.connectionString)
	defer db.Close()

	// Testing the connnection to the postgres DB
	if err := db.Ping(); err != nil {
		log.Fatalf("Error connecting to the database: %v", err)
	}

	// 2. Make sure the migration tabel exists
	_, err := db.Exec(parent.CreateMigrationTable())
	if err != nil {
		log.Fatalf("Error connecting to migration table: %v", err)
	}

	// 3. Import the new files
	err = parent.LoadMigrationFile(scriptpath, db)
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	fmt.Println("All Migrations executed successfully")
	return nil
}

func (parent *PostgresDB) LoadMigrationFile(basePath string, db *sql.DB) error {
	files, err := os.ReadDir(basePath)
	if err != nil {
		log.Fatalf("Could not read the folder: %v", err)
	}

	// Filtering kun *.sql
	var migrations []string
	for _, f := range files {
		if !f.IsDir() && strings.ToLower(filepath.Ext(f.Name())) == ".sql" {
			migrations = append(migrations, f.Name())
		}
	}

	// Sorts i ASC order
	sort.Strings(migrations)

	// Run migration
	for _, m := range migrations {
		// Check if the file exists
		var migrationAlreadyRun bool
		err := db.QueryRow("SELECT EXISTS (SELECT 1 FROM migrations WHERE filename = $1)", m).Scan(&migrationAlreadyRun)
		if err != nil {
			log.Fatalf("Error checking migration table: %v", err)
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

		// Run SQL-script
		_, err = db.ExecContext(parent.ctx, string(sqlBytes))
		if err != nil {
			return fmt.Errorf("error executing SQL in %s: %w", m, err)
		}

		// Insert into the migration table
		_, err = db.Exec("INSERT INTO migrations (filename) VALUES ($1)", m)
		if err != nil {
			return fmt.Errorf("error inserting %s into migrations table: %w", m, err)
		}
	}

	return nil
}

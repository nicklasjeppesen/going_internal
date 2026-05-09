package drivers

import (
	"database/sql"
	"fmt"
	"log"
	"myapp/internal/super/constants"
	types "myapp/internal/super/db/types"
	"myapp/internal/super/util"
	"strconv"
	"strings"
)

// Postgress driver

type PostgresDB struct {
	Conditions    []string
	Params        []any
	IsWhere       bool
	table         string
	withLimit     bool
	limit         int
	shouldOrderBy bool     // Determine if the records shall be order by a column
	orderBy       []string // What column shall be
	offSet        int
	withOffSet    bool
}

func CreatePostgressDB() types.DBCreator {
	var host = util.GetEnv(constants.DB_HOST, "")
	var user = util.GetEnv(constants.DB_USER, "")
	var password = util.GetEnv(constants.DB_PASS, "")
	var dbname = util.GetEnv(constants.DB_DATABASE, "")
	ConnectionString := fmt.Sprintf("host=%s user=%s password=%s dbname=%s sslmode=disable", host, user, password, dbname)
	return types.DBCreator{
		Driver:           &PostgresDB{},
		ConnectionString: ConnectionString,
	}
}

func (parent *PostgresDB) Clone() types.IDrivers {
	return &PostgresDB{}
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

	err := _db.QueryRow(query, values...).Scan(scanArgs...)
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

	row := _db.QueryRow(query, parent.Params...)
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
	err := _db.QueryRow(query, accumaltedValues...)
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
	err := _db.QueryRow(query, parent.Params...)
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

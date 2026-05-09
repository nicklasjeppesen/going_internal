package DB

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

type TestDB struct {
	DB               *sql.DB
	ConnectionString string
}

func (db *TestDB) LoadSchema(schemaSQL string) error {
	_, err := db.DB.Exec(schemaSQL)
	return err
}

func (db *TestDB) LoadJsonData(jsonData []byte, table string) error {

	var data map[string]interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return err
	}

	columns := make([]string, 0, len(data))
	values := make([]interface{}, 0, len(data))
	placeholders := make([]string, 0, len(data))

	for k, v := range data {
		columns = append(columns, k)
		values = append(values, v)
		placeholders = append(placeholders, "?")
	}

	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		table,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)

	_, err := db.DB.Exec(query, values...)
	return err
}

func (db *TestDB) GetConnectionString() string {
	return db.ConnectionString
}

func NewInMemoryDB() (*TestDB, error) {
	//connectionString := ":memory:"
	connectionString := "./testdb.db"

	db, err := sql.Open("sqlite3", connectionString)
	if err != nil {
		return nil, err
	}

	// Check the connection is working properly
	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &TestDB{DB: db, ConnectionString: connectionString}, nil
}

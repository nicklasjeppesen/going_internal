package schema

import (
	"fmt"
	"strings"
)

type Column struct {
	Name         string
	Type         string
	IsNullable   bool
	IsUnique     bool
	DefaultValue interface{}
	IsForeignKey bool
	References   string
	OnDelete     string
	Precision    int // Til decimal
	Scale        int // Til decimal
}

type Blueprint struct {
	TableName string
	Columns   []*Column
}

// Fluent modifiers
func (c *Column) Nullable() *Column               { c.IsNullable = true; return c }
func (c *Column) Unique() *Column                 { c.IsUnique = true; return c }
func (c *Column) Default(val interface{}) *Column { c.DefaultValue = val; return c }

// Kolonne typer
func (b *Blueprint) String(name string) *Column {
	col := &Column{Name: name, Type: "string"}
	b.Columns = append(b.Columns, col)
	return col
}

func (b *Blueprint) Text(name string) *Column {
	col := &Column{Name: name, Type: "text"}
	b.Columns = append(b.Columns, col)
	return col
}

func (b *Blueprint) LongText(name string) *Column {
	col := &Column{Name: name, Type: "longText"}
	b.Columns = append(b.Columns, col)
	return col
}

func (b *Blueprint) Boolean(name string) *Column {
	col := &Column{Name: name, Type: "boolean"}
	b.Columns = append(b.Columns, col)
	return col
}

func (b *Blueprint) DateTime(name string) *Column {
	col := &Column{Name: name, Type: "dateTime"}
	b.Columns = append(b.Columns, col)
	return col
}

func (b *Blueprint) Decimal(name string, precision, scale int) *Column {
	col := &Column{Name: name, Type: "decimal", Precision: precision, Scale: scale}
	b.Columns = append(b.Columns, col)
	return col
}

func (b *Blueprint) JSON(name string) *Column {
	col := &Column{Name: name, Type: "json"}
	b.Columns = append(b.Columns, col)
	return col
}

func (b *Blueprint) ForeignId(name string) *Column {
	col := &Column{Name: name, Type: "foreignId"}
	b.Columns = append(b.Columns, col)
	return col
}

// Constraints og hjælpermetoder
func (c *Column) Constrained() *Column {
	// Simpel logik: user_id -> users table
	table := strings.TrimSuffix(c.Name, "_id") + "s"
	c.IsForeignKey = true
	c.References = table
	return c
}

func (c *Column) OnDelete_(action string) *Column {
	c.OnDelete = action
	return c
}

func (b *Blueprint) Timestamps() {
	b.DateTime("created_at").Nullable()
	b.DateTime("updated_at").Nullable()
}

func (b *Blueprint) SoftDeletes() {
	b.DateTime("deleted_at").Nullable()
}

type Grammar interface {
	Compile(b *Blueprint) string
}

type PostgresGrammar struct{}

func (g PostgresGrammar) Compile(b *Blueprint) string {
	var cols []string
	for _, c := range b.Columns {
		sql := fmt.Sprintf("%s %s", c.Name, g.getType(c))
		if c.IsUnique {
			sql += " UNIQUE"
		}
		if !c.IsNullable {
			sql += " NOT NULL"
		}
		if c.DefaultValue != nil {
			sql += fmt.Sprintf(" DEFAULT %v", c.DefaultValue)
		}
		if c.IsForeignKey {
			sql += fmt.Sprintf(" REFERENCES %s(id)", c.References)
			if c.OnDelete != "" {
				sql += " ON DELETE " + strings.ToUpper(c.OnDelete)
			}
		}
		cols = append(cols, sql)
	}
	return fmt.Sprintf("CREATE TABLE %s (\n  id SERIAL PRIMARY KEY,\n  %s\n);", b.TableName, strings.Join(cols, ",\n  "))
}

func (g PostgresGrammar) getType(c *Column) string {
	switch c.Type {
	case "string":
		return "VARCHAR(255)"
	case "text", "longText":
		return "TEXT"
	case "boolean":
		return "BOOLEAN"
	case "dateTime":
		return "TIMESTAMP"
	case "decimal":
		return fmt.Sprintf("DECIMAL(%d,%d)", c.Precision, c.Scale)
	case "json":
		return "JSONB"
	case "foreignId":
		return "INTEGER"
	default:
		return "VARCHAR(255)"
	}
}

// SQLiteGrammar ville implementere det samme, men returnere f.eks. "INTEGER" for booleans
// og "TEXT" for JSON, da SQLite ikke har indfødte typer til dem.

//USAGE:
/*
func main() {
	blueprint := &Blueprint{TableName: "posts"}

	// Definitionen
	blueprint.String("title")
	blueprint.String("slug").Unique()
	blueprint.Text("excerpt")
	blueprint.LongText("body")
	blueprint.ForeignId("user_id").Constrained().OnDelete("cascade")
	blueprint.ForeignId("category_id").Nullable().Constrained()
	blueprint.Boolean("is_published").Default(false)
	blueprint.DateTime("published_at").Nullable()
	blueprint.Decimal("reading_time", 5, 2).Nullable()
	blueprint.JSON("meta").Nullable()
	blueprint.Timestamps()
	blueprint.SoftDeletes()

	// Generer SQL til Postgres
	grammar := PostgresGrammar{}
	fmt.Println(grammar.Compile(blueprint))
}

Produce this:
CREATE TABLE posts (
  id SERIAL PRIMARY KEY,
  title VARCHAR(255) NOT NULL,
  slug VARCHAR(255) UNIQUE NOT NULL,
  excerpt TEXT NOT NULL,
  body TEXT NOT NULL,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  category_id INTEGER REFERENCES categories(id),
  is_published BOOLEAN NOT NULL DEFAULT false,
  published_at TIMESTAMP,
  reading_time DECIMAL(5,2),
  meta JSONB,
  created_at TIMESTAMP,
  updated_at TIMESTAMP,
  deleted_at TIMESTAMP
);
*/

type SQLiteGrammar struct{}

func (g SQLiteGrammar) Compile(b *Blueprint) string {
	var cols []string
	for _, c := range b.Columns {
		sql := fmt.Sprintf("%s %s", c.Name, g.getType(c))

		// SQLite kræver PRIMARY KEY AUTOINCREMENT på selve id-kolonnen
		if c.Type == "id" {
			sql += " PRIMARY KEY AUTOINCREMENT"
		}

		if c.IsUnique {
			sql += " UNIQUE"
		}

		if !c.IsNullable {
			sql += " NOT NULL"
		}

		if c.DefaultValue != nil {
			sql += fmt.Sprintf(" DEFAULT %v", g.formatDefault(c.DefaultValue))
		}

		// Inline Foreign Keys til SQLite
		if c.IsForeignKey {
			sql += fmt.Sprintf(" REFERENCES %s(id)", c.References)
			if c.OnDelete != "" {
				sql += " ON DELETE " + strings.ToUpper(c.OnDelete)
			}
		}

		cols = append(cols, sql)
	}

	return fmt.Sprintf("CREATE TABLE %s (\n  %s\n);", b.TableName, strings.Join(cols, ",\n  "))
}

func (g SQLiteGrammar) getType(c *Column) string {
	switch c.Type {
	case "id":
		return "INTEGER"
	case "string", "text", "longText", "json", "dateTime":
		return "TEXT"
	case "boolean":
		return "INTEGER" // 0 eller 1
	case "decimal":
		return "NUMERIC"
	case "foreignId":
		return "INTEGER"
	default:
		return "TEXT"
	}
}

func (g SQLiteGrammar) formatDefault(val interface{}) string {
	switch v := val.(type) {
	case bool:
		if v {
			return "1"
		}
		return "0"
	case string:
		return fmt.Sprintf("'%s'", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// Simpler example that work!
/*
// You can edit this code!
// Click here and start typing.
package main

import (
	"fmt"
	"strings"
)

// Column repræsenterer en database-kolonne
type Column struct {
	Name string
	Type string
}

// Blueprint holder styr på alle kolonner for en tabel
type Blueprint struct {
	TableName string
	Columns   []Column
}

func (b *Blueprint) ID() {
	b.Columns = append(b.Columns, Column{Name: "id", Type: "SERIAL PRIMARY KEY"})
}

func (b *Blueprint) String(name string, length int) {
	b.Columns = append(b.Columns, Column{
		Name: name,
		Type: fmt.Sprintf("VARCHAR(%d)", length),
	})
}

func (b *Blueprint) Timestamps() {
	b.Columns = append(b.Columns, Column{Name: "created_at", Type: "TIMESTAMP DEFAULT CURRENT_TIMESTAMP"})
	b.Columns = append(b.Columns, Column{Name: "updated_at", Type: "TIMESTAMP DEFAULT CURRENT_TIMESTAMP"})
}

type Schema struct{}

func (s Schema) Create(tableName string, callback func(table *Blueprint)) {
	blueprint := &Blueprint{TableName: tableName}

	// Her køres callback-funktionen (ligesom Closure i Laravel)
	callback(blueprint)

	// Generer SQL
	var cols []string
	for _, col := range blueprint.Columns {
		cols = append(cols, fmt.Sprintf("%s %s", col.Name, col.Type))
	}

	sql := fmt.Sprintf("CREATE TABLE %s (\n  %s\n);",
		blueprint.TableName,
		strings.Join(cols, ",\n  "),
	)

	// I en rigtig app ville du køre db.Exec(sql) her
	fmt.Println(sql)
}

func main() {
	fmt.Println("Hello, 世界")
	schema := Schema{}

	schema.Create("users", func(table *Blueprint) {
		table.ID()
		table.String("username", 50)
		table.String("email", 255)
		table.Timestamps()
	})
}
*/

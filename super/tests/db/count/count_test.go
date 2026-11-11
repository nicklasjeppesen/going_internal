package count

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/nicklasjeppesen/going_internal/super/db"
	. "github.com/nicklasjeppesen/going_internal/super/tests/db/shared"
)

// Tests for Count, Exists, CountBy, WithCount and RelationCount.

// setup makes the database: authors 1 (Ada), 2 (Bo) and 3 (Cy, with nothing),
// their books, tags and notes. The default connection points at the same file,
// so db.Transaction and the models share one pool.
func setup(t *testing.T) *TestDB {
	t.Helper()
	database, err := NewInMemoryDB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		database.DB.Close()
		for _, f := range []string{"testdb.db", "testdb.db-wal", "testdb.db-shm"} {
			os.Remove(f)
		}
	})
	t.Setenv("DB_CONNECTION", "sqlite")
	t.Setenv("DB_PATH", "./testdb.db")
	for _, statement := range []string{
		schemaSQL,
		`INSERT INTO authors (name) VALUES ('Ada'), ('Bo'), ('Cy')`,
		`INSERT INTO books (author_id, title, published) VALUES (1, 'a1', 1), (1, 'a2', 0), (1, 'a3', 1), (2, 'b1', 0)`,
		`INSERT INTO tags (name, hidden) VALUES ('go', 0), ('sql', 0), ('secret', 1)`,
		`INSERT INTO author_tag (author_id, tag_id, role) VALUES (1, 1, 'lead'), (1, 2, ''), (1, 3, 'lead'), (2, 1, ''), (2, 99, 'lead')`,
		`INSERT INTO notes (body, noteable_id, noteable_type) VALUES ('n1', 2, 'author'), ('n2', 2, 'author'), ('x', 1, 'book')`,
	} {
		if err := database.LoadSchema(statement); err != nil {
			t.Fatal(err)
		}
	}
	return database
}

func TestCountAndExists(t *testing.T) {
	setup(t)
	ctx := context.Background()

	cases := map[string]struct {
		count func() (int64, error)
		want  int64
	}{
		"all":       {func() (int64, error) { return Book{}.DB(ctx).Count() }, 4},
		"where":     {func() (int64, error) { return Book{}.DB(ctx).Where("author_id", 1).Count() }, 3},
		"operator":  {func() (int64, error) { return Book{}.DB(ctx).Where("id", ">", 2).Count() }, 2},
		"two where": {func() (int64, error) { return Book{}.DB(ctx).Where("author_id", 1).Where("published", 1).Count() }, 2},
		"where in":  {func() (int64, error) { return Book{}.DB(ctx).WhereIn("title", []any{"a1", "b1", "zz"}).Count() }, 2},
		"empty in":  {func() (int64, error) { return Book{}.DB(ctx).WhereIn("title", []any{}).Count() }, 0},
		"or":        {func() (int64, error) { return Book{}.DB(ctx).Where("title", "a1").Or("title", "b1").Count() }, 2},
		"order and limit are ignored": {func() (int64, error) {
			return Book{}.DB(ctx).Where("author_id", 1).OrderByDesc("id").Limit(1).Count()
		}, 3},
		"none": {func() (int64, error) { return Book{}.DB(ctx).Where("author_id", 3).Count() }, 0},
	}
	for name, c := range cases {
		got, err := c.count()
		if err != nil || got != c.want {
			t.Errorf("%s: got %d (%v), want %d", name, got, err, c.want)
		}
	}

	if found, err := (Book{}).DB(ctx).Where("author_id", 2).Exists(); err != nil || !found {
		t.Errorf("Exists for author 2: %v %v", found, err)
	}
	if found, err := (Book{}).DB(ctx).Where("author_id", 3).Exists(); err != nil || found {
		t.Errorf("Exists for author 3: %v %v", found, err)
	}
	if _, err := (Book{}).DB(ctx).Where("no_such_column", 1).Count(); err == nil {
		t.Error("Count must return the SQL error")
	}

	perAuthor, err := Book{}.DB(ctx).CountBy("author_id")
	if err != nil || perAuthor[int64(1)] != 3 || perAuthor[int64(2)] != 1 || len(perAuthor) != 2 {
		t.Errorf("CountBy: %v %v", perAuthor, err)
	}
}

var errStop = errors.New("stop")

func TestCountJoinsTheTransaction(t *testing.T) {
	setup(t)

	err := db.Transaction(context.Background(), func(ctx context.Context) error {
		book := Book{}.DB(ctx)
		book.Author_id, book.Title = 3, "c1"
		if _, err := book.Save(); err != nil {
			return err
		}
		// On SQLite, a count outside the transaction would wait for its lock;
		// inside, it sees the uncommitted row.
		if n, err := (Book{}).DB(ctx).Where("author_id", 3).Count(); err != nil || n != 1 {
			t.Errorf("Count in the transaction: %d %v", n, err)
		}
		if found, err := (Book{}).DB(ctx).Where("author_id", 3).Exists(); err != nil || !found {
			t.Errorf("Exists in the transaction: %v %v", found, err)
		}
		return errStop
	})
	if !errors.Is(err, errStop) {
		t.Fatal(err)
	}
	if n, _ := (Book{}).DB(context.Background()).Where("author_id", 3).Count(); n != 0 {
		t.Fatalf("after the rollback: %d", n)
	}
}

// counts is each author's count of relation, by name.
func counts(authors []*Author, relation string) map[string]int64 {
	out := map[string]int64{}
	for _, a := range authors {
		out[a.Name] = a.RelationCount(relation)
	}
	return out
}

func TestWithCountOnSeveralRows(t *testing.T) {
	setup(t)
	ctx := context.Background()

	authors := Author{}.DB(ctx).WithCount("Books", "PublishedBooks", "Tags", "VisibleTags", "LeadTags", "Notes").OrderBy("id").Get()
	if len(authors) != 3 {
		t.Fatalf("authors: %d", len(authors))
	}

	want := map[string]map[string]int64{
		"Books":          {"Ada": 3, "Bo": 1, "Cy": 0}, // HasMany
		"PublishedBooks": {"Ada": 2, "Bo": 0, "Cy": 0}, // HasMany with Where
		"Tags":           {"Ada": 3, "Bo": 1, "Cy": 0}, // BelongsToMany; Bo's tag 99 doesn't exist
		"VisibleTags":    {"Ada": 2, "Bo": 1, "Cy": 0}, // BelongsToMany with Where
		"LeadTags":       {"Ada": 2, "Bo": 0, "Cy": 0}, // BelongsToMany with WherePivot
		"Notes":          {"Ada": 0, "Bo": 2, "Cy": 0}, // HasManyMorph; the book's note isn't Ada's
	}
	for relation, w := range want {
		got := counts(authors, relation)
		for name, n := range w {
			if got[name] != n {
				t.Errorf("%s: %s has %d, want %d", relation, name, got[name], n)
			}
		}
	}

	// The counts agree with what With loads.
	for _, a := range (Author{}).DB(ctx).With("Books").WithCount("Books").Get() {
		if int64(len(a.Books().Items())) != a.RelationCount("Books") {
			t.Errorf("%s: %d books loaded, count %d", a.Name, len(a.Books().Items()), a.RelationCount("Books"))
		}
	}
}

func TestWithCountOnFirstAndZero(t *testing.T) {
	setup(t)
	ctx := context.Background()

	ada := Author{}.DB(ctx).WithCount("Books", "Tags").Where("name", "Ada").First()
	if ada.RelationCount("Books") != 3 || ada.RelationCount("Tags") != 3 {
		t.Fatalf("Ada: books %d, tags %d", ada.RelationCount("Books"), ada.RelationCount("Tags"))
	}
	cy := Author{}.DB(ctx).WithCount("Books", "Tags", "Notes").Where("name", "Cy").First()
	if cy.RelationCount("Books") != 0 || cy.RelationCount("Tags") != 0 || cy.RelationCount("Notes") != 0 {
		t.Fatal("an author with nothing counts 0")
	}
	if ada.RelationCount("NotAsked") != 0 {
		t.Fatal("a relation WithCount wasn't asked for counts 0")
	}

	// No rows: nothing is counted, nothing fails.
	if got := (Author{}).DB(ctx).WithCount("Books").Where("name", "Nobody").Get(); len(got) != 0 {
		t.Fatalf("got %d", len(got))
	}
}

func TestWithCountOfAnUnknownRelationPanics(t *testing.T) {
	setup(t)
	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic")
		}
	}()
	Author{}.DB(context.Background()).WithCount("Nope").Get()
}

package count

import (
	"context"

	. "github.com/nicklasjeppesen/going_internal/super/db"
	drivers "github.com/nicklasjeppesen/going_internal/super/db/drivers"
	relations "github.com/nicklasjeppesen/going_internal/super/db/relationship"
	. "github.com/nicklasjeppesen/going_internal/super/db/types"
)

// The models of the count tests: an author has books (HasMany), published
// books (HasMany with a Where), notes (HasManyMorph) and tags (BelongsToMany
// through author_tag, a pivot table without an id column).

const schemaSQL = `
CREATE TABLE authors (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE books (id INTEGER PRIMARY KEY AUTOINCREMENT, author_id INTEGER NOT NULL DEFAULT 0, title TEXT NOT NULL DEFAULT '',
	published INTEGER NOT NULL DEFAULT 0,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE tags (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL DEFAULT '', hidden INTEGER NOT NULL DEFAULT 0,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE author_tag (author_id INTEGER NOT NULL, tag_id INTEGER NOT NULL,
	role TEXT NOT NULL DEFAULT '');
CREATE TABLE notes (id INTEGER PRIMARY KEY AUTOINCREMENT, body TEXT NOT NULL DEFAULT '',
	noteable_id INTEGER NOT NULL DEFAULT 0, noteable_type TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP);
`

// connection is the test database the models use.
func connection(ctx context.Context) DBCreator {
	return DBCreator{Driver: drivers.CreateSQLite(ctx).Driver, ConnectionString: "./testdb.db"}
}

type Author struct {
	ActiveRecord[*Author]
	Name string
}

func (Author) DB(ctx context.Context) *Author {
	a := &Author{}
	a.Table = "authors"
	a.Columns = Columns{"name": &a.Name}
	a.ParentDB = CreateORMWithCustomDB(ctx, a, connection(ctx))
	return a
}

// Books are all the author's books.
func (a *Author) Books() *relations.HasManyRelation[*Book] {
	return a.HasMany(Book{}.DB(a.GetCtx()))
}

// PublishedBooks are the author's published books: a relation with a Where.
func (a *Author) PublishedBooks() *relations.HasManyRelation[*Book] {
	return a.HasMany(Book{}.DB(a.GetCtx())).Where("published", 1)
}

// Tags are the author's tags through author_tag.
func (a *Author) Tags() *relations.BelongsToManyRelation[*Tag] {
	return a.BelongsToMany(Tag{}.DB(a.GetCtx()))
}

// VisibleTags are the tags that aren't hidden: a BelongsToMany with a Where.
func (a *Author) VisibleTags() *relations.BelongsToManyRelation[*Tag] {
	return a.BelongsToMany(Tag{}.DB(a.GetCtx())).Where("hidden", 0)
}

// LeadTags are the tags with role "lead" on the pivot.
func (a *Author) LeadTags() *relations.BelongsToManyRelation[*Tag] {
	return a.BelongsToMany(Tag{}.DB(a.GetCtx())).WherePivot("role", "lead")
}

// Notes are the author's notes (noteable_type = "author").
func (a *Author) Notes() *relations.HasManyMorphRelation[*Note] {
	return a.HasManyMorph(Note{}.DB(a.GetCtx()))
}

type Book struct {
	ActiveRecord[*Book]
	Author_id int64
	Title     string
	Published int64
}

func (Book) DB(ctx context.Context) *Book {
	b := &Book{}
	b.Table = "books"
	b.Columns = Columns{"author_id": &b.Author_id, "title": &b.Title, "published": &b.Published}
	b.ParentDB = CreateORMWithCustomDB(ctx, b, connection(ctx))
	return b
}

type Tag struct {
	ActiveRecord[*Tag]
	Name   string
	Hidden int64
}

func (Tag) DB(ctx context.Context) *Tag {
	t := &Tag{}
	t.Table = "tags"
	t.Columns = Columns{"name": &t.Name, "hidden": &t.Hidden}
	t.ParentDB = CreateORMWithCustomDB(ctx, t, connection(ctx))
	return t
}

type Note struct {
	ActiveRecord[*Note]
	Body          string
	Noteable_id   int64
	Noteable_type string
}

func (Note) DB(ctx context.Context) *Note {
	n := &Note{}
	n.Table = "notes"
	n.Name = "note" // GetName would singularize "notes" to "not"
	n.Columns = Columns{"body": &n.Body, "noteable_id": &n.Noteable_id, "noteable_type": &n.Noteable_type}
	n.ParentDB = CreateORMWithCustomDB(ctx, n, connection(ctx))
	return n
}

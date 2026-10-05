package db

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/nicklasjeppesen/going_internal/super/db/drivers"
	. "github.com/nicklasjeppesen/going_internal/super/db/types"
)

type pageItem struct {
	ActiveRecord[*pageItem]
	Name string
}

func (_item pageItem) DB(ctx context.Context) *pageItem {
	item := &_item
	item.Table = "items"
	item.Columns = Columns{"name": &item.Name}
	item.ParentDB = CreateORM(ctx, item)
	return item
}

// seedItems creates a SQLite database with n rows: "item 1" … "item n" (id 1 … n).
func seedItems(t *testing.T, n int) context.Context {
	t.Helper()
	path := filepath.Join(t.TempDir(), "page.db")
	t.Setenv("DB_CONNECTION", "sqlite")
	t.Setenv("DB_PATH", path)
	drivers.ResetPools()
	t.Cleanup(drivers.ResetPools)

	conn, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= n; i++ {
		if _, err := conn.Exec(`INSERT INTO items (name) VALUES (?)`, fmt.Sprintf("item %d", i)); err != nil {
			t.Fatal(err)
		}
	}
	return context.Background()
}

func names(items []*pageItem) string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = strings.TrimPrefix(it.Name, "item ")
	}
	return strings.Join(out, ",")
}

func TestPaginate(t *testing.T) {
	ctx := seedItems(t, 7)
	get := func(page int) Page[*pageItem] {
		return new(pageItem).DB(ctx).OrderBy("id").Paginate(page, 3)
	}

	p := get(1)
	if names(p.Items) != "1,2,3" || p.Page != 1 || !p.HasNext || p.NextPage() != 2 || p.PrevPage() != 0 {
		t.Fatalf("page 1 = %s %+v", names(p.Items), p)
	}
	p = get(3)
	if names(p.Items) != "7" || p.HasNext || p.NextPage() != 0 || p.PrevPage() != 2 {
		t.Fatalf("page 3 = %s %+v", names(p.Items), p)
	}
	if p = get(0); p.Page != 1 || names(p.Items) != "1,2,3" {
		t.Fatalf("page 0 = %d %s, want page 1", p.Page, names(p.Items))
	}
	if p = get(9); len(p.Items) != 0 || p.HasNext {
		t.Fatalf("page past the end = %s %+v", names(p.Items), p)
	}
}

// A full last page must not promise a next (empty) page.
func TestPaginateFullLastPage(t *testing.T) {
	ctx := seedItems(t, 6)
	p := new(pageItem).DB(ctx).OrderBy("id").Paginate(2, 3)
	if names(p.Items) != "4,5,6" || p.HasNext || p.NextPage() != 0 {
		t.Fatalf("full last page = %s %+v", names(p.Items), p)
	}
}

func TestBeforeAndAfter(t *testing.T) {
	ctx := seedItems(t, 10)

	newest := new(pageItem).DB(ctx).Before("id", 0, 4)
	if names(newest.Items) != "10,9,8,7" || !newest.HasMore || newest.NextCursor != 7 {
		t.Fatalf("Before(0) = %s %+v", names(newest.Items), newest)
	}
	older := new(pageItem).DB(ctx).Before("id", newest.NextCursor, 4)
	if names(older.Items) != "6,5,4,3" || !older.HasMore {
		t.Fatalf("Before(7) = %s", names(older.Items))
	}
	oldest := new(pageItem).DB(ctx).Before("id", 3, 4)
	if names(oldest.Items) != "2,1" || oldest.HasMore || oldest.NextCursor != 1 {
		t.Fatalf("Before(3) = %s %+v", names(oldest.Items), oldest)
	}
	if names(oldest.Items.Reverse()) != "1,2" {
		t.Fatal("Reverse")
	}

	newer := new(pageItem).DB(ctx).Where("name", "!=", "item 6").After("id", 4, 3)
	if names(newer.Items) != "5,7,8" || !newer.HasMore || newer.NextCursor != 8 {
		t.Fatalf("After(4) with a filter = %s %+v", names(newer.Items), newer)
	}
	if last := new(pageItem).DB(ctx).After("id", 8, 3); names(last.Items) != "9,10" || last.HasMore {
		t.Fatalf("After(8) = %s", names(last.Items))
	}
	if none := new(pageItem).DB(ctx).After("id", 10, 3); len(none.Items) != 0 || none.NextCursor != 0 {
		t.Fatalf("After(10) = %+v", none)
	}
}

package db

import (
	"fmt"
	"strconv"

	. "github.com/nicklasjeppesen/going_internal/super/collections"
)

// ─── Offset pagination (page 1, 2, 3 …) ───────────────────────────────────────
// The ORM only knows numbers: which page and how many per page. Reading ?page
// from the request and building links is the HTTP layer's job (request.PageNumber,
// request.PageURL, request.Paginated) – like Kaminari in Rails and Paginator in Laravel.

// Page is one page of a query: the models (with eager loads) and whether there
// are more pages.
//
//	page := new(Post).DB(ctx).OrderByDesc("id").Paginate(request.PageNumber(), 20)
//	{{range .Posts.Items}} … {{end}}
type Page[T any] struct {
	Items   Collection[T]
	Page    int // 1, 2, 3 …
	PerPage int
	HasNext bool // there is at least one more row
}

// Paginate fetches page page (1 if page < 1) with perPage rows. One extra row
// is fetched so HasNext is correct, even when the last page is completely full.
// Set the ordering (OrderBy) before Paginate.
func (parent *ParentDB[T]) Paginate(page, perPage int) Page[T] {
	page = max(page, 1)
	parent.Limit(perPage + 1)
	parent.OffSet((page - 1) * perPage)
	items := parent.Get()

	p := Page[T]{Items: items, Page: page, PerPage: perPage}
	if len(items) > perPage {
		p.Items, p.HasNext = items[:perPage], true
	}
	return p
}

// NextPage is the number of the next page, or 0 when there are no more.
func (p Page[T]) NextPage() int {
	if p.HasNext {
		return p.Page + 1
	}
	return 0
}

// PrevPage is the number of the previous page, or 0 on the first page.
func (p Page[T]) PrevPage() int {
	if p.Page > 1 {
		return p.Page - 1
	}
	return 0
}

// ─── Keyset pagination (cursor) ───────────────────────────────────────────────
// For lists that grow while you read them (chat, activity): a page is fetched
// from a column (usually id) instead of an offset, so new rows don't shift
// the pages and nothing is seen twice or skipped.

// CursorPage is one page from Before or After.
type CursorPage[T any] struct {
	Items      Collection[T] // in the query's order (Before: descending, After: ascending)
	HasMore    bool          // there are more rows in the same direction
	NextCursor int64         // the column's value in the last row: pass it as cursor for the next page
}

// Before fetches up to size rows where column < cursor (from the top if
// cursor <= 0), ordered by column descending – e.g. messages older than a given one.
//
//	older := new(Message).DB(ctx).Where("chat_id", id).WithUser().Before("id", before, 20)
//	older.Items.Reverse() // oldest first for display
func (parent *ParentDB[T]) Before(column string, cursor int64, size int) CursorPage[T] {
	if cursor > 0 {
		parent.Where(column, "<", cursor)
	}
	parent.OrderByDesc(column)
	return parent.cursorPage(column, size)
}

// After fetches up to size rows where column > cursor, ordered by column
// ascending – e.g. messages newer than a given one.
func (parent *ParentDB[T]) After(column string, cursor int64, size int) CursorPage[T] {
	parent.Where(column, ">", cursor)
	parent.OrderBy(column)
	return parent.cursorPage(column, size)
}

func (parent *ParentDB[T]) cursorPage(column string, size int) CursorPage[T] {
	parent.Limit(size + 1)
	items := parent.Get()

	p := CursorPage[T]{Items: items}
	if len(items) > size {
		p.Items, p.HasMore = items[:size], true
	}
	if len(p.Items) > 0 {
		p.NextCursor = cursorValue(p.Items[len(p.Items)-1], column)
	}
	return p
}

// cursorValue reads column (e.g. "id") from a model as int64.
func cursorValue(item any, column string) int64 {
	model, ok := item.(interface{ Value(string) (any, error) })
	if !ok {
		return 0
	}
	value, err := model.Value(column)
	if err != nil {
		return 0
	}
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	}
	n, _ := strconv.ParseInt(fmt.Sprint(value), 10, 64)
	return n
}

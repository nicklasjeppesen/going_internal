package request

import (
	"strconv"

	"github.com/nicklasjeppesen/going_internal/super/db"
)

// ─── Pagination: the HTTP part ────────────────────────────────────────────────
// The ORM paginates with numbers (db.Paginate(page, perPage)); here ?page is read
// from the request, and links are built from the current URL.
//
//	files := chat.Files(request.PageNumber(), 30)
//	nextURL := request.PageURL(files.NextPage()) // "" when there are no more

// PageNumber is ?page as a number: 1 if it is missing or invalid.
func (r *Requestbase) PageNumber() int {
	if n, err := strconv.Atoi(r.R.URL.Query().Get("page")); err == nil && n > 1 {
		return n
	}
	return 1
}

// PageURL is the current path with page=n; the other query parameters are kept.
// Empty when n <= 0 (e.g. page.NextPage() on the last page).
func (r *Requestbase) PageURL(n int) string {
	if n <= 0 {
		return ""
	}
	query := r.R.URL.Query()
	query.Set("page", strconv.Itoa(n))
	return r.R.URL.Path + "?" + query.Encode()
}

// Paginated is the page as a JSON response with links, e.g. for an API:
//
//	return Response.PrintJson(request.Paginated(page))
func (r *Requestbase) Paginated[T any](page db.Page[T]) map[string]any {
	return map[string]any{
		"current_page":   page.Page,
		"data":           page.Items.ToJson(),
		"per_page":       page.PerPage,
		"has_next":       page.HasNext,
		"path":           r.R.URL.Path,
		"first_page_url": r.PageURL(1),
		"next_page_url":  r.PageURL(page.NextPage()),
		"prev_page_url":  r.PageURL(page.PrevPage()),
	}
}

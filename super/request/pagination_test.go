package request

import (
	"net/http/httptest"
	"testing"

	"github.com/nicklasjeppesen/going_internal/super/collections"
	"github.com/nicklasjeppesen/going_internal/super/db"
)

func req(url string) *Requestbase {
	return &Requestbase{R: httptest.NewRequest("GET", url, nil)}
}

func TestPageNumber(t *testing.T) {
	cases := map[string]int{"/x": 1, "/x?page=3": 3, "/x?page=0": 1, "/x?page=-2": 1, "/x?page=abc": 1}
	for url, want := range cases {
		if got := req(url).PageNumber(); got != want {
			t.Errorf("PageNumber(%s) = %d, want %d", url, got, want)
		}
	}
}

func TestPageURL(t *testing.T) {
	r := req("/chats/5/files?q=a+b&page=1")
	if got := r.PageURL(2); got != "/chats/5/files?page=2&q=a+b" {
		t.Errorf("PageURL(2) = %q, want the other query parameters kept", got)
	}
	if got := r.PageURL(0); got != "" {
		t.Errorf("PageURL(0) = %q, want empty (no such page)", got)
	}
}

func TestPaginated(t *testing.T) {
	page := db.Page[string]{Items: collections.Collection[string]{"a", "b"}, Page: 2, PerPage: 2, HasNext: true}
	m := req("/api/items?page=2").Paginated(page)
	if m["current_page"] != 2 || m["has_next"] != true || len(m["data"].([]any)) != 2 ||
		m["next_page_url"] != "/api/items?page=3" || m["prev_page_url"] != "/api/items?page=1" || m["first_page_url"] != "/api/items?page=1" {
		t.Fatalf("Paginated = %+v", m)
	}
	last := db.Page[string]{Items: collections.Collection[string]{"c"}, Page: 1, PerPage: 2}
	if m := req("/api/items").Paginated(last); m["next_page_url"] != "" || m["prev_page_url"] != "" {
		t.Fatalf("single page = %+v", m)
	}
}

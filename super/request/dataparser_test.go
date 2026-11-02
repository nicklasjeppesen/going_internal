package request

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseDataToOrmQueryFormBody(t *testing.T) {
	r := httptest.NewRequest("QUERY", "/search?sort=desc", strings.NewReader("name=alice&page=2"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var out struct {
		Name string `form:"name"`
		Page int    `form:"page"`
		Sort string `form:"sort"`
	}
	if err := parseDataToOrm(r, &out); err != nil {
		t.Fatalf("parseDataToOrm: %v", err)
	}
	if out.Name != "alice" || out.Page != 2 || out.Sort != "desc" {
		t.Fatalf("parsed %+v, want name=alice page=2 sort=desc", out)
	}
}

func TestParseDataToOrmQueryJSONBody(t *testing.T) {
	r := httptest.NewRequest("QUERY", "/search", strings.NewReader(`{"name":"alice","page":2}`))
	r.Header.Set("Content-Type", "application/json")

	var out struct {
		Name string `json:"name"`
		Page int    `json:"page"`
	}
	if err := parseDataToOrm(r, &out); err != nil {
		t.Fatalf("parseDataToOrm: %v", err)
	}
	if out.Name != "alice" || out.Page != 2 {
		t.Fatalf("parsed %+v, want name=alice page=2", out)
	}
}

func TestFormValueReadsQueryBody(t *testing.T) {
	r := &Requestbase{R: httptest.NewRequest("QUERY", "/search?sort=desc", strings.NewReader("name=alice"))}
	r.R.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if got := r.FormValue("name"); got != "alice" {
		t.Errorf(`FormValue("name") = %q, want "alice"`, got)
	}
	if got := r.FormValue("sort"); got != "desc" {
		t.Errorf(`FormValue("sort") = %q, want "desc"`, got)
	}
}

func TestFormValueStillReadsGETQuery(t *testing.T) {
	r := &Requestbase{R: httptest.NewRequest(http.MethodGet, "/search?sort=desc", nil)}
	if got := r.FormValue("sort"); got != "desc" {
		t.Errorf(`FormValue("sort") = %q, want "desc"`, got)
	}
}

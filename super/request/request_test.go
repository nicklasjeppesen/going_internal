package request

import "testing"

func TestParamReadsQuery(t *testing.T) {
	r := req("/chats/5/files?kind=images&q=a+b")
	if got := r.Param("kind"); got != "images" {
		t.Errorf(`Param("kind") = %q, want "images"`, got)
	}
	if got := r.Param("q"); got != "a b" {
		t.Errorf(`Param("q") = %q, want the decoded value "a b"`, got)
	}
	if got := r.Param("missing"); got != "" {
		t.Errorf(`Param("missing") = %q, want ""`, got)
	}
}

func TestParamPrefersRouteParameter(t *testing.T) {
	r := req("/people/letters/A?letter=B")
	r.R.SetPathValue("letter", "A")
	if got := r.Param("letter"); got != "A" {
		t.Errorf(`Param("letter") = %q, want the route value "A"`, got)
	}
}

func TestParamInt(t *testing.T) {
	cases := map[string]int{"/x?offset=20": 20, "/x": 0, "/x?offset=abc": 0, "/x?offset=-3": -3}
	for url, want := range cases {
		if got := req(url).ParamInt("offset"); got != want {
			t.Errorf("ParamInt(offset) for %s = %d, want %d", url, got, want)
		}
	}
	if got := req("/x?before=9007199254740993").ParamInt64("before"); got != 9007199254740993 {
		t.Errorf("ParamInt64(before) = %d", got)
	}
	if got := req("/x").ParamInt64("before"); got != 0 {
		t.Errorf("ParamInt64(missing) = %d, want 0", got)
	}
}

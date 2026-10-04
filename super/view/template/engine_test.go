package template

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nicklasjeppesen/going_internal/super/constants"
)

// writeViews creates internal/resources/views in a temp dir (the engine reads
// that path relative to the working directory) and changes into it.
func writeViews(t *testing.T, files map[string]string) {
	t.Helper()
	dir := t.TempDir()
	views := filepath.Join(dir, "internal", "resources", "views")
	if err := os.MkdirAll(views, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(views, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
}

// A template with a syntax error must not silently stop the loading of the
// templates after it (it used to, which surfaced later as "no such template").
func TestBrokenTemplateDoesNotHideTheOthers(t *testing.T) {
	writeViews(t, map[string]string{
		"a_broken.tmpl": `{{define "broken"}}{{if}}{{end}}`,
		"b_good.tmpl":   `{{define "good"}}ok{{end}}`,
		"c_later.tmpl":  `{{define "later"}}later{{end}}`,
	})

	engine := New()

	for _, name := range []string{"good", "later"} {
		if engine.templates.Lookup(name) == nil {
			t.Errorf("template %q not loaded", name)
		}
	}
	if len(engine.parseErrors) != 1 || !strings.Contains(engine.parseErrors[0].Error(), "a_broken.tmpl") {
		t.Fatalf("parseErrors = %v, want one error naming a_broken.tmpl", engine.parseErrors)
	}
}

// In debug mode the 500 page names the broken file, so "could not find view"
// points at the real cause.
func TestMissingViewShowsParseErrorInDebug(t *testing.T) {
	writeViews(t, map[string]string{
		"page.tmpl": `{{define "page"}}{{if}}{{end}}`,
	})
	t.Setenv(constants.APP_Debug, "true")

	view := TemplateView{BaseView: "debug-test-" + t.Name()}.View("page")
	rec := httptest.NewRecorder()
	view(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "page.tmpl") {
		t.Fatalf("body does not name the broken file:\n%s", body)
	}
}

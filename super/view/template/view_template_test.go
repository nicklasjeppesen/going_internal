package template

import (
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/nicklasjeppesen/going_internal/super/constants"
	sessionstore "github.com/nicklasjeppesen/going_internal/super/session"
)

func TestMain(m *testing.M) {
	os.Setenv(constants.APP_Key, "test-key-0123456789abcdef0123456")
	os.Exit(m.Run())
}

// Templates are parsed lazily on first use; concurrent first requests must
// not race on the cache (run with -race) and must share one parse.
func TestTemplatesForConcurrentFirstUse(t *testing.T) {
	view := TemplateView{BaseView: "concurrent-test"}
	results := make([]any, 20)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = templatesFor(view)
		}(i)
	}
	wg.Wait()
	for i := range results {
		if results[i] != results[0] {
			t.Fatal("templatesFor returned different template sets")
		}
	}
}

// Errors, old input and flash messages are read from one session cookie and
// consumed with a single save.
func TestGetDataConsumesSessionOnce(t *testing.T) {
	// A session cookie holding all three values
	setup := httptest.NewRecorder()
	setupReq := httptest.NewRequest(http.MethodGet, "/", nil)
	session := sessionstore.Get(setupReq)
	session.Values[constants.Errors] = `{"email":["required"]}`
	session.Values[constants.Old] = `{"email":"a@b.dk"}`
	session.Values[constants.Flash] = `{"msg":"saved"}`
	if err := session.Save(setupReq, setup); err != nil {
		t.Fatal(err)
	}
	cookie := setup.Result().Cookies()[0]

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	data := getData(req, rec, "page")

	for _, key := range []string{"haserrors", "hasold", "hasflash"} {
		if data[key] != true {
			t.Errorf("%s = %v, want true", key, data[key])
		}
	}
	if old, _ := data[constants.Old].(map[string]any); old["email"] != "a@b.dk" {
		t.Errorf("old = %v", data[constants.Old])
	}
	if n := len(rec.Header()["Set-Cookie"]); n != 1 {
		t.Fatalf("Set-Cookie headers = %d, want 1", n)
	}

	// Next request with the updated cookie: everything consumed, nothing saved
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.AddCookie(rec.Result().Cookies()[0])
	rec2 := httptest.NewRecorder()
	data2 := getData(req2, rec2, "page")
	for _, key := range []string{"haserrors", "hasold", "hasflash"} {
		if data2[key] != false {
			t.Errorf("second request: %s = %v, want false", key, data2[key])
		}
	}
	if n := len(rec2.Header()["Set-Cookie"]); n != 0 {
		t.Fatalf("second request: Set-Cookie headers = %d, want 0", n)
	}
}

package response

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/nicklasjeppesen/going_internal/super/constants"
)

func TestMain(m *testing.M) {
	os.Setenv(constants.APP_Key, "test-key-0123456789abcdef0123456")
	os.Exit(m.Run())
}

// Apps keep one package-level Response; WithErrors/With must not change it,
// or one user's errors end up in the next user's session.
func TestSharedResponseIsNotChanged(t *testing.T) {
	shared := NewResponse()
	withErrors := shared.WithErrors(map[string]string{"message": "Invalid credentials"})
	withFlash := shared.With(map[string]string{"msg": "saved"})

	if len(shared.ErrorMessage()) != 0 || len(shared.FlashData()) != 0 {
		t.Fatalf("shared response changed: errors=%v flash=%v", shared.ErrorMessage(), shared.FlashData())
	}
	if len(withErrors.ErrorMessage()) != 1 || len(withFlash.FlashData()) != 1 {
		t.Fatal("returned responses lost their data")
	}
	if got := shared.WithErrors("boom").ErrorMessage()["error"]; len(got) != 1 {
		t.Fatalf("string error = %v", got)
	}
	if len(shared.ErrorMessage()) != 0 {
		t.Fatal("string WithErrors changed the shared response")
	}
}

func loginPost(password string) *http.Request {
	form := url.Values{"email": {"anna@example.com"}, "password": {password},
		"password_confirmation": {password}, "csrf_token": {"abc"}}
	r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ParseForm()
	return r
}

func TestOldInputLeavesOutSecrets(t *testing.T) {
	inputs := GetInputs(loginPost("hunter2"))
	if inputs["email"] != "anna@example.com" {
		t.Errorf("email missing from old input: %v", inputs)
	}
	for _, field := range []string{"password", "password_confirmation", "csrf_token"} {
		if _, ok := inputs[field]; ok {
			t.Errorf("%s stored as old input", field)
		}
	}
}

// The session cookie is encrypted and HttpOnly: nothing in it can be read in
// the browser, not even the (non-secret) old email.
func TestSessionCookieIsEncryptedAndHttpOnly(t *testing.T) {
	rec := httptest.NewRecorder()
	NewResponse().WithErrors(map[string]string{"message": "Invalid credentials"}).Back()(rec, loginPost("hunter2"))

	var cookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == constants.Session_info {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("no session cookie set")
	}
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie flags: HttpOnly=%v Secure=%v SameSite=%v", cookie.HttpOnly, cookie.Secure, cookie.SameSite)
	}
	raw, _ := base64.URLEncoding.DecodeString(cookie.Value + strings.Repeat("=", (4-len(cookie.Value)%4)%4))
	for _, plain := range []string{"anna@example.com", "Invalid credentials", "hunter2"} {
		if strings.Contains(string(raw), plain) || strings.Contains(decodeInner(raw), plain) {
			t.Errorf("cookie reveals %q", plain)
		}
	}
}

// decodeInner decodes the value part of a securecookie ("date|value|mac").
func decodeInner(raw []byte) string {
	parts := strings.Split(string(raw), "|")
	if len(parts) < 2 {
		return ""
	}
	inner, _ := base64.URLEncoding.DecodeString(parts[1] + strings.Repeat("=", (4-len(parts[1])%4)%4))
	return string(inner)
}

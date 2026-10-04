package middleware

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	constants "github.com/nicklasjeppesen/going_internal/super/constants"
	"github.com/nicklasjeppesen/going_internal/super/request"
)

func multipartPost(token string, fileSize int) *http.Request {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	w.WriteField("csrf_token", token)
	f, _ := w.CreateFormFile("file", "x.bin")
	f.Write(bytes.Repeat([]byte("a"), fileSize))
	w.Close()
	r := httptest.NewRequest(http.MethodPost, "/upload", &body)
	r.Header.Set("Content-Type", w.FormDataContentType())
	r.AddCookie(&http.Cookie{Name: constants.Csrf_token, Value: "good"})
	return r
}

func csrfServe(r *http.Request, limit int64) int {
	rec := httptest.NewRecorder()
	r = request.LimitBody(rec, r, limit)
	CsrfMiddleware(func(req *request.Requestbase) { req.W.WriteHeader(http.StatusOK) })(&request.Requestbase{W: rec, R: r})
	return rec.Code
}

func TestCsrfMultipart(t *testing.T) {
	if got := csrfServe(multipartPost("good", 100), 1<<20); got != 200 {
		t.Errorf("valid token: %d, want 200", got)
	}
	if got := csrfServe(multipartPost("bad", 100), 1<<20); got != 401 {
		t.Errorf("wrong token: %d, want 401", got)
	}
	if got := csrfServe(multipartPost("good", 2<<20), 1<<20); got != 413 {
		t.Errorf("body over the limit: %d, want 413", got)
	}
}

func TestCsrfJSONTooLarge(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"csrf_token":"good","x":"`+strings.Repeat("a", 2<<20)+`"}`))
	r.Header.Set("Content-Type", "application/json")
	r.AddCookie(&http.Cookie{Name: constants.Csrf_token, Value: "good"})
	if got := csrfServe(r, 1<<20); got != 413 {
		t.Errorf("JSON over the limit: %d, want 413", got)
	}
}

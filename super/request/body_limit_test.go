package request

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBodyLimitCanBeRaisedBeforeReading(t *testing.T) {
	body := strings.Repeat("x", 100)

	r := LimitBody(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)), 10)
	if _, err := io.ReadAll(r.Body); err == nil {
		t.Fatal("read past the limit")
	}

	r = LimitBody(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)), 10)
	if !SetBodyLimit(r, 1000) {
		t.Fatal("SetBodyLimit refused before reading")
	}
	if data, err := io.ReadAll(r.Body); err != nil || len(data) != 100 {
		t.Fatalf("raised limit: %d bytes, err %v", len(data), err)
	}
	if SetBodyLimit(r, 5000) {
		t.Fatal("SetBodyLimit accepted after reading started")
	}
}

package unittest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "github.com/nicklasjeppesen/going_internal/super/response"
)

func TestStatusBadRequest(t *testing.T) {
	fail := &Fail{}
	handler := fail.StatusBadRequest("bad request")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "bad request") {
		t.Errorf("expected body to contain 'bad request', got %q", rec.Body.String())
	}
}

func TestStatusUnauthorized(t *testing.T) {
	fail := &Fail{}
	handler := fail.StatusUnauthorized("unauthorized")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "unauthorized") {
		t.Errorf("expected body to contain 'unauthorized', got %q", rec.Body.String())
	}
}

func TestStatusForbidden(t *testing.T) {
	fail := &Fail{}
	handler := fail.StatusForbidden("forbidden")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "forbidden") {
		t.Errorf("expected body to contain 'forbidden', got %q", rec.Body.String())
	}
}

func TestStatusNotFound(t *testing.T) {
	fail := &Fail{}
	handler := fail.StatusNotFound("not found")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "not found") {
		t.Errorf("expected body to contain 'not found', got %q", rec.Body.String())
	}
}

func TestStatusInternalServerError(t *testing.T) {
	fail := &Fail{}
	handler := fail.StatusInternalServerError("server error")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "server error") {
		t.Errorf("expected body to contain 'server error', got %q", rec.Body.String())
	}
}

func TestFailReturnsHandlerFunc(t *testing.T) {
	fail := &Fail{}

	tests := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"BadRequest", fail.StatusBadRequest("msg")},
		{"Unauthorized", fail.StatusUnauthorized("msg")},
		{"Forbidden", fail.StatusForbidden("msg")},
		{"NotFound", fail.StatusNotFound("msg")},
		{"InternalServerError", fail.StatusInternalServerError("msg")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.handler == nil {
				t.Errorf("%s returned nil handler", tt.name)
			}
		})
	}
}

func TestStatusBadRequestEmptyMessage(t *testing.T) {
	fail := &Fail{}
	handler := fail.StatusBadRequest("")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

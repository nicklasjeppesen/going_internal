package unittest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "github.com/nicklasjeppesen/going_internal/super/response"
)

func TestNewResponse(t *testing.T) {
	resp := NewResponse()
	if resp == nil {
		t.Fatal("NewResponse() returned nil")
	}
}

func TestNewResponseIsEmpty(t *testing.T) {
	resp := NewResponse()
	if len(resp.ErrorMessage()) != 0 {
		t.Errorf("NewResponse().errorMessage should be empty, got %v", resp.ErrorMessage())
	}
	if len(resp.FlashData()) != 0 {
		t.Errorf("NewResponse().flashData should be empty, got %v", resp.FlashData())
	}
}

func TestWithErrorsString(t *testing.T) {
	resp := NewResponse()
	result := resp.WithErrors("something went wrong")

	if result != resp {
		t.Error("WithErrors should return the same Response instance for chaining")
	}

	errors := resp.ErrorMessage()
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errors))
	}
	msgs, ok := errors["error"]
	if !ok {
		t.Fatal("expected key 'error' not found")
	}
	if len(msgs) != 1 || msgs[0] != "something went wrong" {
		t.Errorf("expected [\"something went wrong\"], got %v", msgs)
	}
}

func TestWithErrorsMapStringString(t *testing.T) {
	resp := NewResponse()
	errs := map[string]string{
		"email":    "required",
		"password": "min 8 chars",
	}
	resp.WithErrors(errs)

	result := resp.ErrorMessage()
	if len(result) != 2 {
		t.Fatalf("expected 2 errors, got %d", len(result))
	}
	if result["email"][0] != "required" {
		t.Errorf("expected 'required' for email, got %v", result["email"])
	}
	if result["password"][0] != "min 8 chars" {
		t.Errorf("expected 'min 8 chars' for password, got %v", result["password"])
	}
}

func TestWithErrorsMapStringSlice(t *testing.T) {
	resp := NewResponse()
	errs := map[string][]string{
		"email": {"required", "invalid format"},
		"name":  {"too short"},
	}
	resp.WithErrors(errs)

	result := resp.ErrorMessage()
	if len(result) != 2 {
		t.Fatalf("expected 2 error fields, got %d", len(result))
	}
	if len(result["email"]) != 2 {
		t.Errorf("expected 2 errors for email, got %d", len(result["email"]))
	}
	if result["name"][0] != "too short" {
		t.Errorf("expected 'too short' for name, got %v", result["name"])
	}
}

func TestWithErrorsPanicsOnUnsupportedType(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("WithErrors should panic on unsupported type")
		}
	}()

	resp := NewResponse()
	resp.WithErrors(123)
}

func TestWithSetsFlashData(t *testing.T) {
	resp := NewResponse()
	flash := map[string]string{"success": "Profile updated"}
	result := resp.With(flash)

	if result != resp {
		t.Error("With should return the same Response instance for chaining")
	}

	data := resp.FlashData()
	if data["success"] != "Profile updated" {
		t.Errorf("expected 'Profile updated', got %v", data["success"])
	}
}

func TestWithOverwritesFlashData(t *testing.T) {
	resp := NewResponse()
	resp.With(map[string]string{"success": "first"})
	resp.With(map[string]string{"info": "second"})

	data := resp.FlashData()
	if len(data) != 1 {
		t.Errorf("expected 1 flash entry after overwrite, got %d", len(data))
	}
	if data["info"] != "second" {
		t.Errorf("expected 'second', got %v", data["info"])
	}
}

func TestPrintJsonWritesJSON(t *testing.T) {
	resp := NewResponse()
	user := struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}{ID: 1, Name: "Alice"}

	handler := resp.PrintJson(user)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %v", contentType)
	}
	body := strings.TrimSpace(rec.Body.String())
	if body != `{"id":1,"name":"Alice"}` {
		t.Errorf("unexpected body: %v", body)
	}
}

func TestPrintJsonWithStatusCode(t *testing.T) {
	resp := NewResponse()
	handler := resp.PrintJson(map[string]string{"error": "not found"}, http.StatusNotFound)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

func TestPrintJsonWithoutStatusCode(t *testing.T) {
	resp := NewResponse()
	handler := resp.PrintJson("hello")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected default status 200, got %d", rec.Code)
	}
}

func TestPrintWritesRawOutput(t *testing.T) {
	resp := NewResponse()
	handler := resp.Print("hello world")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	body := strings.TrimSpace(rec.Body.String())
	if body != "hello world" {
		t.Errorf("expected 'hello world', got %q", body)
	}
}

func TestPrintWritesStruct(t *testing.T) {
	resp := NewResponse()
	data := map[string]int{"a": 1}
	handler := resp.Print(data)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	body := strings.TrimSpace(rec.Body.String())
	if !strings.Contains(body, "1") {
		t.Errorf("expected body to contain '1', got %q", body)
	}
}

func TestRedirectSends302(t *testing.T) {
	resp := NewResponse()
	handler := resp.Redirect("/dashboard")

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("expected status 302, got %d", rec.Code)
	}
	location := rec.Header().Get("Location")
	if location != "/dashboard" {
		t.Errorf("expected Location '/dashboard', got %q", location)
	}
}

func TestBackRedirectsToReferer(t *testing.T) {
	resp := NewResponse()
	handler := resp.Back()

	req := httptest.NewRequest(http.MethodGet, "/current", nil)
	req.Header.Set("Referer", "/previous")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected status 303, got %d", rec.Code)
	}
	location := rec.Header().Get("Location")
	if location != "/previous" {
		t.Errorf("expected Location '/previous', got %q", location)
	}
}

func TestBackRedirectsToRootWhenNoReferer(t *testing.T) {
	resp := NewResponse()
	handler := resp.Back()

	req := httptest.NewRequest(http.MethodGet, "/current", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	location := rec.Header().Get("Location")
	if location != "/" {
		t.Errorf("expected Location '/', got %q", location)
	}
}

func TestGetInputsExtractsFormValues(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/?name=foo&email=bar", nil)
	req.Form = map[string][]string{
		"name":  {"foo"},
		"email": {"bar"},
	}

	inputs := GetInputs(req)

	if inputs["name"] != "foo" {
		t.Errorf("expected name=foo, got %v", inputs["name"])
	}
	if inputs["email"] != "bar" {
		t.Errorf("expected email=bar, got %v", inputs["email"])
	}
}

func TestGetInputsEmptyForm(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Form = map[string][]string{}

	inputs := GetInputs(req)

	if len(inputs) != 0 {
		t.Errorf("expected empty map, got %v", inputs)
	}
}

func TestGetInputsLastValueWins(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Form = map[string][]string{
		"color": {"red", "blue"},
	}

	inputs := GetInputs(req)

	if inputs["color"] != "blue" {
		t.Errorf("expected last value 'blue', got %v", inputs["color"])
	}
}

func TestChainingWithErrorsAndWith(t *testing.T) {
	resp := NewResponse()
	result := resp.WithErrors("error").With(map[string]string{"success": "done"})

	if result != resp {
		t.Error("chaining should return the same instance")
	}
	if len(resp.ErrorMessage()) != 1 {
		t.Error("expected 1 error")
	}
	if resp.FlashData()["success"] != "done" {
		t.Error("expected flash data 'done'")
	}
}

func TestToJSONWithNil(t *testing.T) {
	result, err := ToJSON(nil)
	if err != nil {
		t.Errorf("ToJSON(nil) should not error, got %v", err)
	}
	if result != "false" {
		t.Errorf("ToJSON(nil) = %q, want %q", result, "false")
	}
}

func TestToJSONWithInt(t *testing.T) {
	result, err := ToJSON(42)
	if err != nil {
		t.Errorf("ToJSON(42) should not error, got %v", err)
	}
	if result != "42" {
		t.Errorf("ToJSON(42) = %q, want %q", result, "42")
	}
}

func TestToJSONWithBool(t *testing.T) {
	result, err := ToJSON(true)
	if err != nil {
		t.Errorf("ToJSON(true) should not error, got %v", err)
	}
	if result != "true" {
		t.Errorf("ToJSON(true) = %q, want %q", result, "true")
	}
}

func TestToJSONWithMap(t *testing.T) {
	data := map[string]int{"a": 1, "b": 2}
	result, err := ToJSON(data)
	if err != nil {
		t.Errorf("ToJSON(map) should not error, got %v", err)
	}
	if !strings.Contains(result, `"a":1`) || !strings.Contains(result, `"b":2`) {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestToJSONWithSlice(t *testing.T) {
	data := []string{"x", "y"}
	result, err := ToJSON(data)
	if err != nil {
		t.Errorf("ToJSON(slice) should not error, got %v", err)
	}
	if result != `["x","y"]` {
		t.Errorf("ToJSON(slice) = %q, want %q", result, `["x","y"]`)
	}
}

func TestToJSONWithNestedStruct(t *testing.T) {
	type Inner struct {
		Value string `json:"value"`
	}
	type Outer struct {
		Inner Inner `json:"inner"`
		Name  string `json:"name"`
	}
	data := Outer{Inner: Inner{Value: "test"}, Name: "outer"}
	result, err := ToJSON(data)
	if err != nil {
		t.Errorf("ToJSON(nested) should not error, got %v", err)
	}
	if !strings.Contains(result, `"inner"`) || !strings.Contains(result, `"value":"test"`) {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestPrintJsonWithNilValue(t *testing.T) {
	resp := NewResponse()
	handler := resp.PrintJson(nil)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	body := strings.TrimSpace(rec.Body.String())
	if body != "false" {
		t.Errorf("expected 'false', got %q", body)
	}
}

func TestPrintJsonWithEmptyStruct(t *testing.T) {
	resp := NewResponse()
	handler := resp.PrintJson(struct{}{})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	body := strings.TrimSpace(rec.Body.String())
	if body != "{}" {
		t.Errorf("expected '{}', got %q", body)
	}
}

package request

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	form "github.com/go-playground/form"
)

func parseDataToOrm(r *http.Request, target any) error {

	contentType := r.Header.Get("Content-Type")
	switch {

	case strings.Contains(contentType, "application/json"):
		return decodeJSONBody(r, target)

	case strings.Contains(contentType, "application/x-www-form-urlencoded"):
		return decodeFormBody(r, target)

	case strings.Contains(contentType, "multipart/form-data"):
		return decodeMultipartForm(r, target)

	default:
		return fmt.Errorf("unsupported content type")
	}
}

func decodeJSONBody(r *http.Request, target any) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("failed reading body: %w", err)
	}

	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("invalid JSON body with error: %w", err)
	}
	return nil
}

func decodeFormBody(r *http.Request, target any) error {
	if err := parseQueryForm(r); err != nil {
		return err
	}
	if err := r.ParseForm(); err != nil {
		return err
	}

	decoder := form.NewDecoder()
	return decoder.Decode(target, r.Form)
}

func decodeMultipartForm(r *http.Request, target any) error {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return err
	}

	decoder := form.NewDecoder()
	return decoder.Decode(target, r.Form)
}

// parseQueryForm makes the body of a QUERY request available through r.Form.
// net/http only parses a urlencoded body for POST, PUT and PATCH, but QUERY is
// defined as a safe method that may carry a body. Calling it for other methods
// is a no-op.
func parseQueryForm(r *http.Request) error {
	if r.Method != "QUERY" {
		return nil
	}

	contentType := r.Header.Get("Content-Type")

	if strings.Contains(contentType, "multipart/form-data") {
		return r.ParseMultipartForm(32 << 20)
	}

	if !strings.Contains(contentType, "application/x-www-form-urlencoded") {
		return nil
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("failed reading body: %w", err)
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	bodyValues, err := url.ParseQuery(string(body))
	if err != nil {
		return fmt.Errorf("invalid form body: %w", err)
	}

	if err := r.ParseForm(); err != nil {
		return err
	}

	// Like ParseForm, body values come before URL query values.
	values := bodyValues
	for key, vs := range r.Form {
		values[key] = append(values[key], vs...)
	}
	r.Form = values
	return nil
}

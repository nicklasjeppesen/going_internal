package request

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
		return fmt.Errorf("invalid JSON body")
	}

	/*
		if err := validateRequiredJSONFields(body, reflectType); err != nil {
			return err
		}
	*/

	return nil
}

func decodeFormBody(r *http.Request, target any) error {
	decoder := form.NewDecoder()
	err := decoder.Decode(target, r.Form)
	return err
}

func decodeMultipartForm(r *http.Request, target any) error {
	decoder := form.NewDecoder()
	err := decoder.Decode(target, r.Form)
	return err
}

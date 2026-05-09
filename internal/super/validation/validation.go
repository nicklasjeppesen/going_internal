package validation

import (
	"fmt"
	"reflect"

	"github.com/go-playground/validator/v10"
)

func Validate[T any](t T) (bool, string) {
	validate := validator.New()
	err := validate.Struct(t)
	if err != nil {
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			var errorMessages []string

			valType := reflect.TypeOf(t)
			// Hvis t er en pointer, tag elementet
			if valType.Kind() == reflect.Ptr {
				valType = valType.Elem()
			}

			for _, fieldError := range validationErrors {
				// Use reflection to find the JSON field name
				if field, found := valType.FieldByName(fieldError.StructField()); found {
					jsonTag := field.Tag.Get("json")
					// Split to remove any options like omitempty
					jsonFieldName := jsonTag
					if commaIndex := len(jsonTag); commaIndex != -1 {
						jsonFieldName = jsonTag[:commaIndex]
					}

					errorMessage := fmt.Sprintf(
						"Field '%s' failed validation '%s', expected: %v; ",
						jsonFieldName,
						fieldError.ActualTag(),
						fieldError.Param(),
					)
					errorMessages = append(errorMessages, errorMessage)
				}
			}
			return true, fmt.Sprintf("Validation errors: %s", errorMessages)
		}
		// Handle unexpected errors from validator
		return true, "Validation failed due to an unexpected error"
	}
	return false, ""

}

type Validator interface {
	Validate() error
}

func Customvalidation[T any](v T) error {
	if val, ok := any(v).(Validator); ok {
		return val.Validate()
	}

	if val, ok := any(&v).(Validator); ok {
		return val.Validate()
	}

	return nil
}

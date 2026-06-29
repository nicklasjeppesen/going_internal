package validation

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

func Validate[T any](t T) (bool, map[string][]string) {
	validate := validator.New()
	err := validate.Struct(t)
	if err != nil {

		errorsMap := make(map[string][]string)

		if validationErrors, ok := err.(validator.ValidationErrors); ok {

			valType := reflect.TypeOf(t)
			// Hvis t er en pointer, tag elementet
			if valType.Kind() == reflect.Ptr {
				valType = valType.Elem()
			}

			for _, fieldError := range validationErrors {
				fieldName := fieldError.StructField()
				// Use reflection to find the JSON field name
				if field, found := valType.FieldByName(fieldError.StructField()); found {
					jsonTag := field.Tag.Get("json")
					// Split to remove any options like omitempty

					/*jsonFieldName := jsonTag
					if commaIndex := len(jsonTag); commaIndex != -1 {
						jsonFieldName = jsonTag[:commaIndex]
					}*/
					if jsonTag != "" && jsonTag != "-" {
						// Split for at fjerne options som f.eks. 'omitempty'
						jsonFieldName := strings.Split(jsonTag, ",")[0]
						if jsonFieldName != "" {
							fieldName = jsonFieldName
						}
					}

					errorMessage := fmt.Sprintf(
						"failed validation '%s', expected: %v; ",
						fieldError.ActualTag(),
						fieldError.Param(),
					)
					errorsMap[fieldName] = append(errorsMap[fieldName], errorMessage)
				}
			}
			return false, errorsMap
		}
		// Handle unexpected errors from validator
		errorsMap["$global"] = append(errorsMap["$global"], "Validation failed due to an unexpected error")
		return false, errorsMap
	}
	return true, map[string][]string{}

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

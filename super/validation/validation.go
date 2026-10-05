package validation

import (
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

// validate is the shared validator instance for the whole application.
// Custom rules are registered via RegisterValidation, which must happen
// before the server starts receiving requests.
var validate = validator.New()

// RegisterValidation lets consumers (template projects) add their own
// validation rules. Not thread-safe with respect to concurrent Validate() calls,
// so call it at startup, never from a handler.
func RegisterValidation(tag string, fn validator.Func, callEvenIfNull ...bool) error {
	return validate.RegisterValidation(tag, fn, callEvenIfNull...)
}

func Validate[T any](t T) (bool, map[string][]string) {
	err := validate.Struct(t)
	if err != nil {
		errorsMap := make(map[string][]string)

		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			valType := reflect.TypeOf(t)
			if valType.Kind() == reflect.Ptr {
				valType = valType.Elem()
			}

			structName := valType.Name()

			for _, fieldError := range validationErrors {
				fieldName := fieldError.StructField()

				if field, found := valType.FieldByName(fieldError.StructField()); found {
					jsonTag := field.Tag.Get("json")
					if jsonTag != "" && jsonTag != "-" {
						jsonFieldName := strings.Split(jsonTag, ",")[0]
						if jsonFieldName != "" {
							fieldName = jsonFieldName
						}
					}

					message := resolveMessage(structName, fieldError)
					errorsMap[fieldName] = append(errorsMap[fieldName], message)
				}
			}
			return false, errorsMap
		}

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

// resolveMessage finds the most specific message via rules.json
func resolveMessage(structName string, fe validator.FieldError) string {
	// 1. Check for a field-specific message
	if msg, ok := GetFieldMessage(structName, fe.StructField(), fe.Tag()); ok {
		return replacePlaceholders(msg, fe)
	}

	// 2. Check for a default message
	if msg, ok := GetDefaultMessage(fe.Tag()); ok {
		return replacePlaceholders(msg, fe)
	}

	// 3. Fallback
	return "Ugyldig værdi for regel '" + fe.Tag() + "'"
}

// replacePlaceholders replaces {param} and {field} with actual values
func replacePlaceholders(msg string, fe validator.FieldError) string {
	msg = strings.ReplaceAll(msg, "{param}", fe.Param())
	msg = strings.ReplaceAll(msg, "{field}", fe.Field())
	return msg
}

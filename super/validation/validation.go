package validation

import (
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

// validate er den delte validator-instans for hele applikationen.
// Registrering af custom rules sker via RegisterValidation, og skal ske
// før serveren begynder at modtage requests.
var validate = validator.New()

// RegisterValidation lader forbrugere (template-projekter) tilføje deres
// egne valideringsregler. Ikke thread-safe i forhold til samtidige Validate()-kald,
// så kald denne ved opstart, aldrig fra en handler.
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

// resolveMessage finder den mest specifikke besked via rules.json
func resolveMessage(structName string, fe validator.FieldError) string {
	// 1. Tjek felt-specifik besked
	if msg, ok := GetFieldMessage(structName, fe.StructField(), fe.Tag()); ok {
		return replacePlaceholders(msg, fe)
	}

	// 2. Tjek default besked
	if msg, ok := GetDefaultMessage(fe.Tag()); ok {
		return replacePlaceholders(msg, fe)
	}

	// 3. Fallback
	return "Ugyldig værdi for regel '" + fe.Tag() + "'"
}

// replacePlaceholders udskifter {param} og {field} med faktiske værdier
func replacePlaceholders(msg string, fe validator.FieldError) string {
	msg = strings.ReplaceAll(msg, "{param}", fe.Param())
	msg = strings.ReplaceAll(msg, "{field}", fe.Field())
	return msg
}

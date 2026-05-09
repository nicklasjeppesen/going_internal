package response

import (
	"encoding/json"
	"fmt"
	"net/http"

	. "github.com/nicklasjeppesen/going_internal/super/util"
)

//
//------------------------------------------------------------------------
// 					Response
//------------------------------------------------------------------------
//
// Response is responsible for generate output to a client
// It receive its input from the controller class, and return it to the user
//

// Struct to handle different kind of response, a controller can return.
type Response struct {
}

// Print a struct to Json
//
// if struct field has hidden:true tag, it will be ignored.
// if the type is is a struct that has a the method: ToJson, this method
// will be called by reflect before casting by Json.Marshal method
func (c *Response) PrintJson(_v any) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		output, _ := ToJSON(_v)
		fmt.Fprintln(w, output)
	}
}

// Recursive function for printing a struct
/*
// DEPRECATED
func ToMap(v reflect.Value) any {

	if !v.IsValid() {
		return ""
	}

	switch v.Kind() {

	case
		reflect.Bool,
		reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16,
		reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16,
		reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64,
		reflect.Complex64, reflect.Complex128:
		return v.Interface()

	case reflect.Ptr, reflect.Interface:
		if v.IsNil() {
			return nil
		}
		return ToMap(v.Elem())

	case reflect.Slice, reflect.Array:
		var arr []any
		for i := 0; i < v.Len(); i++ {
			arr = append(arr, ToMap(v.Index(i)))
		}
		return arr

	// Used for Maps, sort of Routes
	case reflect.Map:
		result := make(map[string]any)
		for _, value := range v.MapKeys() {

			key := fmt.Sprint(value.Interface())
			mapIndex := v.MapIndex(value)
			result[key] = ToMap(mapIndex)
		}
		return result

	case reflect.Struct:
		result := make(map[string]any)

		var dd = time.Time{}
		if v.Type() == reflect.TypeOf(dd) {
			t := v.Interface().(time.Time)
			return t.String()
		}

		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			fieldType := t.Field(i)
			fieldValue := v.Field(i)

			// Skip unexported fields
			if fieldType.PkgPath != "" {
				continue
			}

			// Skip Field with tag hidden
			if fieldType.Tag.Get("hidden") == "true" {
				continue
			}

			jsonTag := fieldType.Tag.Get("json")
			if jsonTag == "" {
				jsonTag = fieldType.Name

			}

			result[jsonTag] = ToMap(fieldValue)

		}
		return result

	default:
		return v.Interface()
	}
}
*/

func ToJSON(s any) (string, error) {

	s = HasJsonFunc(s)
	b, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (c *Response) Print(_v any) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, _v)
	}
}

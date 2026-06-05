package util

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"time"
)

//-----------------------------------------------------------------//
// 							Struct to map						   //
//-----------------------------------------------------------------//
//
// This file is responsible transform a struct into a map,
// So Go json.Marshall can transform it to a struct
// This file also checked if a struct, implement ToJson function,
// The function return a map of the type: map[string]any

// This function is used if
func HasJsonFunc(v any) any {
	if v == nil {
		return false
	}
	t := reflect.TypeOf(v)
	// Hvis det er en pointer, unwrap den
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	// Checking, and printing

	method := reflect.ValueOf(v).MethodByName("ToJson")
	if method.IsValid() {
		return method.Call(nil)[0].Interface()
	}

	if t.Kind() == reflect.Slice {
		list := []any{}
		fieldValue := reflect.ValueOf(v)
		for i := 0; i < fieldValue.Len(); i++ {

			current := fieldValue.Index(i)
			methodName := current.MethodByName("ToJson")
			if methodName.IsValid() {
				jsonReadyVal := methodName.Call(nil)[0].Interface()
				list = append(list, jsonReadyVal)
			} else {
				list = append(list, current.Interface())
			}
		}
		return list
	}
	return v
}

// Newer version from above, to handle multiple same instances
func GetFieldValue(fieldMeta reflect.StructField, fieldVal reflect.Value, ignore []string, flatten []string, visited map[uintptr]bool) any {
	if visited == nil {
		visited = make(map[uintptr]bool)
	}

	// 1. handles ppointer and interfaces
	kind := fieldVal.Kind()
	if kind == reflect.Ptr || kind == reflect.Interface {
		if fieldVal.IsNil() {
			return ""
		}

		// Tracking pointers in a inifinity loop
		if kind == reflect.Ptr {
			ptr := fieldVal.Pointer()
			if visited[ptr] {
				return ""
			}

			// Mark as visit, run recursive and remove mark again.
			// if to different node pointing to same value, the second will wrongly be marked as visit.
			visited[ptr] = true
			result := GetFieldValue(fieldMeta, fieldVal.Elem(), ignore, flatten, visited)
			delete(visited, ptr)
			return result
		}

		// For interfaces, vi caling elem, without tracking the current interface
		return GetFieldValue(fieldMeta, fieldVal.Elem(), ignore, flatten, visited)
	}

	// 2. Handle primitive types
	switch kind {
	case reflect.Bool, reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64,
		reflect.Complex64, reflect.Complex128:
		return fieldVal.Interface()

	case reflect.Slice, reflect.Array:
		var arr []any
		for i := 0; i < fieldVal.Len(); i++ {
			// Vi hadle visited down to each element
			arr = append(arr, GetFieldValue(fieldMeta, fieldVal.Index(i), ignore, flatten, visited))
		}
		return arr

	case reflect.Map:
		return fieldVal.Interface()

	case reflect.Struct:
		return handleStruct(fieldMeta, fieldVal, ignore, flatten, visited)

	default:
		return ""
	}
}

func handleStruct(fieldMeta reflect.StructField, fieldVal reflect.Value, ignore []string, flatten []string, visited map[uintptr]bool) any {
	if fieldMeta.Type == reflect.TypeOf(time.Time{}) {
		return fieldVal.Interface()
	}
	hidden := fieldMeta.Tag.Get("hidden")
	if hidden != "" {
		fmt.Println(fieldMeta)
	}

	// Caling struct_to_map with tracked map
	return Struct_to_map(fieldVal, ignore, flatten, visited)
}

func Struct_to_map(x reflect.Value, ignore []string, flatten []string, visited map[uintptr]bool) map[string]any {
	if visited == nil {
		visited = make(map[uintptr]bool)
	}

	results := map[string]any{}

	// Sørg for at vi arbejder med den faktiske struct
	v := x
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return nil
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		fieldMeta := t.Field(i)

		if fieldMeta.PkgPath != "" || fieldMeta.Tag.Get("hidden") != "" || slices.Contains(ignore, fieldMeta.Name) {
			continue
		}

		name := fieldMeta.Tag.Get("json")
		if name == "-" {
			continue
		}

		if name == "" {
			name = fieldMeta.Name
		}

		fieldVal := v.Field(i)
		// Send visited mappet videre her
		value := GetFieldValue(fieldMeta, fieldVal, ignore, flatten, visited)

		if slices.Contains(flatten, fieldMeta.Name) {
			if valueMap, ok := value.(map[string]any); ok {
				maps.Copy(results, valueMap)
			}
		} else {
			results[name] = value
		}
	}
	return results
}

package util

import (
	"unicode"
)

type Jsonable_ interface {
	ToJson() []any
}

type JsonableSingle interface {
	ToJson() map[string]any
}

// LowerFirst gør det første bogstav i en streng lille
func LowerFirst(s string) string {
	if s == "" {
		return ""
	}

	// Vi konverterer til runes for at håndtere UTF-8 korrekt
	r := []rune(s)
	r[0] = unicode.ToLower(r[0])

	return string(r)
}

func MapToJson(object map[string][]any) map[string]any {

	relations := map[string]any{}
	for key, relation := range object {
		key = LowerFirst(key)

		if j, ok := any(relation).(Jsonable_); ok {
			customData := j.ToJson()
			relations[key] = customData
		} else if j, ok := any(relation[0]).(Jsonable_); ok {
			customData := j.ToJson()
			relations[key] = customData
		} else if j, ok := any(relation[0]).(JsonableSingle); ok {
			customdata := j.ToJson()
			relations[key] = customdata

		}
	}
	return relations
}

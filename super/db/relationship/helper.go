package relationship

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"

	. "github.com/nicklasjeppesen/going_internal/super/db/types"
)

// Loader is implemented by every relationship loader (BelongsTo, HasMany, etc.),
// which can be resolved by name and eager-loaded.
type Loader interface {
	Load()
	LoadMany(parents []ISystemFields, relationkey string)
}

// LoadSingle resolves the relation method with the given name on a single
// parent and eager-loads it via Load().
func LoadSingle(child any, relationkey string) {
	if loader := relationLoader(child, relationkey); loader != nil {
		loader.Load()
	}
}

func relationLoader(child any, relationkey string) Loader {
	v := reflect.ValueOf(child)
	if !v.IsValid() {
		return nil
	}
	method := v.MethodByName(relationkey)
	if !method.IsValid() {
		return nil
	}
	results := method.Call(nil)
	if len(results) == 0 {
		return nil
	}
	loader, ok := results[0].Interface().(Loader)
	if !ok {
		return nil
	}
	return loader
}

// parentKey returns the parent's value for localKey (normally "id"), which a
// child gets as its foreign key. Holder is the empty child prototype the
// relation was built with, so the id must never be taken from there.
func parentKey(parent ISystemFields, localKey string) (any, error) {
	if localKey == "" {
		localKey = parent.PrimaryKeyName()
	}
	value, err := parent.Value(localKey)
	if err != nil {
		return nil, fmt.Errorf("relationship: parent has no column %q: %w", localKey, err)
	}
	if value == nil {
		return nil, fmt.Errorf("relationship: parent's %q is empty", localKey)
	}
	return value, nil
}

func removeTrailingS(input string) string {
	// queries -> query
	if strings.HasSuffix(strings.ToLower(input), "ies") {
		var newInput = input[:len(input)-3]
		return newInput + "y"
	}

	if strings.HasSuffix(strings.ToLower(input), "s") {
		return input[:len(input)-1]
	}
	return input
}

func PivotTableName(a, b string) string {
	a = strings.ToLower(a)
	b = strings.ToLower(b)

	if a > b {
		a, b = b, a
	}
	return a + "_" + b
}

// Get the name of the caller method, used to get the relationship name
func CallerMethodName() string {
	pc, _, _, ok := runtime.Caller(3)
	if !ok {
		return ""
	}
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return ""
	}

	// e.g.: "main.(*User).Company"
	full := fn.Name()
	parts := strings.Split(full, ".")
	return parts[len(parts)-1]
}

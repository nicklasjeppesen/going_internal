package relationship

import (
	"runtime"
	"strings"
)

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

	// fx: "main.(*User).Company"
	full := fn.Name()
	parts := strings.Split(full, ".")
	return parts[len(parts)-1]
}

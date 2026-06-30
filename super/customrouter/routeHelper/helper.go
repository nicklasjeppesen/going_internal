// Package routeHelper provides utilities for parsing, manipulating, and
// replacing placeholders within URL and routing paths.
package routeHelper

import (
	"fmt"
	"regexp"
	"strings"
)

// CountBracedParams counts the number of placeholders wrapped in curly braces (e.g., "{id}")
// within the given path string.
func CountBracedParams(path string) int {
	re := regexp.MustCompile(`\{[^}]+\}`)
	matches := re.FindAllString(path, -1)
	return len(matches)
}

// ReplaceBracedParams replaces curly brace placeholders in a path with a slice of string values.
// The values are replaced in the order they appear in the path.
//
// It returns an error if the number of placeholders does not match the number of values provided.
func ReplaceBracedParams(path string, values []string) (string, error) {
	re := regexp.MustCompile(`\{[^}]+\}`)
	matches := re.FindAllString(path, -1)

	if len(matches) != len(values) {
		return "", fmt.Errorf("number of placeholders (%d) does not match number of values (%d)", len(matches), len(values))
	}

	result := path
	for i, match := range matches {
		result = regexp.MustCompile(regexp.QuoteMeta(match)).ReplaceAllString(result, values[i])
	}

	return result, nil
}

// ConvertToStrings converts a slice of any types (interfaces) into a slice of strings
// using fmt.Sprintf's default "%v" formatting.
func ConvertToStrings(interfaces []any) []string {

	strings := make([]string, len(interfaces))
	for i, v := range interfaces {
		var s = fmt.Sprintf("%v", v)
		strings[i] = s
	}
	return strings
}

// ReplaceURLPlaceholders takes a map of named routes and replaces named placeholders (e.g., "{id}")
// with their corresponding values from the provided values map.
//
// If a placeholder key is missing from the values map, the function skips that specific route
// and returns an error.
//
// Example:
//
//	urls := map[string]string{
//		"show":          "/user/{id}/",
//		"showWorkspace": "/user/{id}/workspace/{workspace_id}",
//	}
//	values := map[string]string{
//		"id":           "1",
//		"workspace_id": "44",
//	}
//	result, err := ReplaceURLPlaceholders(urls, values)
func ReplaceURLPlaceholders(urls map[string]string, values map[string]string) (map[string]string, error) {

	re := regexp.MustCompile(`\{([^}]+)\}`) // matcher {key}
	var newResult = make(map[string]string)
	var err error

	for namedRoute, url := range urls {

		//var currentKey string
		updatedURL := re.ReplaceAllStringFunc(url, func(m string) string {
			key := strings.Trim(m, "{}")

			val, ok := values[key]
			if !ok {
				err = fmt.Errorf("Missing value for key: %s", key)
				return m
			}
			return fmt.Sprintf("%v", val)
		})

		if err != nil {
			//return nil, err
			continue
		}
		newResult[namedRoute] = updatedURL
	}

	return newResult, err
}

// CollectValuesByPrefix filters a map of routes and returns a new map containing
// only the entries where the route name starts with the specified prefix.
func CollectValuesByPrefix(AllRouteNamesAndUrls map[string]string, prefix string) map[string]string {
	var routeNamesAndUrls = make(map[string]string)
	for routeName, url := range AllRouteNamesAndUrls {
		if strings.HasPrefix(routeName, prefix) {
			routeNamesAndUrls[routeName] = url
		}
	}
	return routeNamesAndUrls
}

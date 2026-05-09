package route

import (
	"fmt"
	"regexp"
	"strings"
)

func CountBracedParams(path string) int {
	re := regexp.MustCompile(`\{[^}]+\}`)
	matches := re.FindAllString(path, -1)
	return len(matches)
}

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

func ConvertToStrings(interfaces []any) []string {

	strings := make([]string, len(interfaces))
	for i, v := range interfaces {
		var s = fmt.Sprintf("%v", v)
		strings[i] = s
	}
	return strings
}

// ReplaceURLPlaceholders erstatter placeholders i URL'er med værdier fra en map.
// Returnerer fejl hvis en nøgle mangler.
//
//	example:
//
//	urls := []string{
//			"show: /user/{id}/",
//			"showWorkspace: /user/{id}/workspace/{workspace_id}",
//			"showBank: /user/{id}/bank/{bank_id}",
//		}
//		values := map[string]string{
//			"id":           "1",
//			"workspace_id": "44",
//			 "bank_id": "5",
//		}
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

// CollectValuesByPrefix
// return a map where all routes name is equal to a given prefix
func CollectValuesByPrefix(AllRouteNamesAndUrls map[string]string, prefix string) map[string]string {
	var routeNamesAndUrls = make(map[string]string)
	for routeName, url := range AllRouteNamesAndUrls {
		if strings.HasPrefix(routeName, prefix) {
			routeNamesAndUrls[routeName] = url
		}
	}
	return routeNamesAndUrls
}

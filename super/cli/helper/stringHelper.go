package helper

import "strings"

func FirstUpper(s string) string {
	s = strings.ToLower(s)
	if s == "" {
		return s
	}

	r := []rune(s)
	r[0] = []rune(strings.ToUpper(string(r[0])))[0]
	return string(r)
}

func AllLower(s string) string {
	return strings.ToLower(s)
}

func TextToMultiplum(text string) string {
	text = strings.TrimSpace(text)
	if len(text) < 2 {
		if text == "y" {
			return "ies"
		}
		if text == "s" {
			return text
		}
		return text + "s"
	}

	//
	if strings.HasSuffix(text, "y") {
		lastLetterIdx := len(text) - 1
		nextlastLetterIdx := string(text[lastLetterIdx-1])

		if strings.ContainsAny(nextlastLetterIdx, "aeiouAEIOU") {
			return text + "s"
		}

		return text[:lastLetterIdx] + "ies"
	}

	if strings.HasSuffix(text, "s") {
		return text
	}

	return text + "s"
}

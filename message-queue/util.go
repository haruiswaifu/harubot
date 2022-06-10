package messageQueue

import "unicode"

func IsAllLower(s string) bool {
	for _, r := range s {
		if !unicode.IsLower(r) && unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

func FilterEmptyWords(words []string) []string {
	nonEmptyWords := []string{}
	for _, word := range words {
		if word != "" {
			nonEmptyWords = append(nonEmptyWords, word)
		}
	}
	return nonEmptyWords
}

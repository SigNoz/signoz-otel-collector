package keycheck

import (
	"regexp"
	"strings"
	"unicode"
)

// Pre-compile regex patterns
var (
	uuidRegex      = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	hexRegex       = regexp.MustCompile(`(?i)^[0-9a-f]{16,}$`)
	base64Regex    = regexp.MustCompile(`^[A-Za-z0-9+/]{16,}={0,2}$`)
	timestampRegex = regexp.MustCompile(`^\d{13}$`)
)

func IsRandomKey(key string) bool {
	length := len(key)

	// Very long keys - let's consider random by default
	if length > 256 {
		return true
	}

	// Simple lowercase keys with reasonable length are likely meaningful
	if length <= 15 && isAlphaLower(key) {
		return false
	}

	// Keys with underscores/hyphens and mostly letters are likely meaningful
	if length <= 25 && strings.ContainsAny(key, "_-") && isMostlyLetters(key) {
		return false
	}

	// Long mixed case strings without digits are likely random
	if length > 15 && hasUpperLowerDigit(key) {
		return true
	}

	// Use a single loop to check all segments
	start := 0
	for i := 0; i <= len(key); i++ {
		if i == len(key) || key[i] == '.' {
			segment := key[start:i]
			// Skip empty segments
			if len(segment) == 0 {
				start = i + 1
				continue
			}

			switch {
			case uuidRegex.MatchString(segment):
				return true
			case hexRegex.MatchString(segment):
				return true
			case base64Regex.MatchString(segment):
				return true
			case timestampRegex.MatchString(segment):
				return true
			case len(segment) > 12 && !containsVowels(segment):
				return true
			case len(segment) > 16 && hasUpperLowerDigit(segment):
				return true
			}
			start = i + 1
		}
	}

	return false
}

func isMostlyLetters(s string) bool {
	count := 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			count++
		}
	}
	return float64(count)/float64(len(s)) > 0.7
}

// isAlphaLower checks if a string contains only lowercase letters
func isAlphaLower(s string) bool {
	for _, r := range s {
		if !unicode.IsLower(r) && !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

func containsVowels(s string) bool {
	for _, r := range s {
		switch unicode.ToLower(r) {
		case 'a', 'e', 'i', 'o', 'u':
			return true
		}
	}
	return false
}

// The function hasUpperLowerDigit is sufficient for checking if a string has both upper and lower case letters, as well as digits.
// Therefore, the hasUpperLower function is redundant and can be removed.
func hasUpperLowerDigit(s string) bool {
	var hasUpper, hasLower, hasDigit bool
	for _, r := range s {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}
	return hasUpper && hasLower && hasDigit
}

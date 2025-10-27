package keycheck

import (
	"regexp"
	"strings"
	"unicode"
)

// Constants for key length thresholds
const (
	MaxKeyLength    = 256 // Keys longer than this are considered random
	ShortKeyLength  = 15  // Keys shorter than this with only lowercase are considered meaningful
	MediumKeyLength = 25  // Keys shorter than this with underscores/hyphens are considered meaningful
	LetterThreshold = 0.7 // Threshold for considering a string as mostly letters
)

// Pre-compile regex patterns
var (
	uuidRegex      = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	hexRegex       = regexp.MustCompile(`(?i)^[0-9a-f]{16,}$`)
	base64Regex    = regexp.MustCompile(`^[A-Za-z0-9+/]{16,}={0,2}$`)
	timestampRegex = regexp.MustCompile(`^\d{13}$`)
)

// IsRandomKey determines if a key appears to be randomly generated.
// It checks for various patterns that typically indicate random keys:
// - UUIDs
// - Hex strings
// - Base64 encoded strings
// - Timestamps
func IsRandomKey(key string) bool {
	length := len(key)

	// Very long keys are considered random by default
	if length > MaxKeyLength {
		return true
	}

	// Simple lowercase keys with reasonable length are likely meaningful
	if length <= ShortKeyLength && isAlphaLower(key) {
		return false
	}

	// Keys with underscores/hyphens and mostly letters are likely meaningful
	if length <= MediumKeyLength && strings.ContainsAny(key, "_-") && isMostlyLetters(key) {
		return false
	}

	// Process each segment of the key (split by dots)
	return processKeySegments(key)
}

// processKeySegments checks each segment of a key for random patterns
func processKeySegments(key string) bool {
	start := 0
	for i := 0; i <= len(key); i++ {
		if i == len(key) || key[i] == '.' {
			segment := key[start:i]
			if len(segment) == 0 {
				start = i + 1
				continue
			}

			if isRandomSegment(segment) {
				return true
			}
			start = i + 1
		}
	}
	return false
}

// isRandomSegment checks if a single segment appears to be randomly generated
func isRandomSegment(segment string) bool {
	switch {
	case len(segment) > MaxKeyLength:
		return true
	case uuidRegex.MatchString(segment):
		return true
	case hexRegex.MatchString(segment):
		return true
	case isBase64String(segment):
		return true
	case timestampRegex.MatchString(segment):
		return true
	case isULID(segment):
		return true
	}
	return false
}

// isBase64String checks if a string is a valid base64 encoded string
func isBase64String(s string) bool {
	if !containsNonAlpha(s) {
		return false
	}
	return base64Regex.MatchString(s)
}

// isMostlyLetters checks if a string consists mostly of letters
func isMostlyLetters(s string) bool {
	count := 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			count++
		}
	}
	return float64(count)/float64(len(s)) > LetterThreshold
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

// containsNonAlpha checks if a string contains any non-alphabetic characters
func containsNonAlpha(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return true
		}
	}
	return false
}


// isULID checks if string matches ULID pattern
func isULID(s string) bool {
	// ULID is 26 characters, base32 encoded
	if len(s) != 26 {
		return false
	}

	// Check if it's base32 (A-Z, 0-9, excluding I, L, O, U)
	for _, char := range s {
		if !((char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9')) {
			return false
		}
		// Exclude I, L, O, U
		if char == 'I' || char == 'L' || char == 'O' || char == 'U' {
			return false
		}
	}

	return true
}

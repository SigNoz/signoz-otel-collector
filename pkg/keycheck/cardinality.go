package keycheck

import (
	"slices"
	"strings"
)

var (
	// The whitelist of allowed symbols only (no letters or numbers)
	// Note: Backticks can never be allowed to exist in a key from User's source
	JSONKeyAllowedSymbols   = []string{"_", ".", ":", "@", "-"}
	BacktickRequiredSymbols = []string{":", "@", "-"}
)

func IsCardinal(key string) bool {
	// whole string cases
	switch {
	case containsDigits(key):
		return true
	case hasNonAllowedSymbols(key):
		return true
	}

	return processKeySegments(key)
}

func IsBacktickRequired(key string) bool {
	return strings.ContainsAny(key, strings.Join(BacktickRequiredSymbols, ""))
}

func CleanBackticks(key string) string {
	return strings.ReplaceAll(key, "`", "")
}

// containsDigits checks if string contains digits
func containsDigits(s string) bool {
	if len(s) == 0 {
		return false
	}

	// Check if any character is a digit
	// TODO: check for better patterns which are okay to be considered as non cardinal across customer ecosystem
	for _, char := range s {
		if char >= '0' && char <= '9' {
			return true
		}
	}
	return false
}

func hasNonAllowedSymbols(s string) bool {
	// Check each character in the string
	for _, char := range s {
		// Allow letters (a-z, A-Z)
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') {
			continue
		}
		// Allow numbers (0-9)
		if char >= '0' && char <= '9' {
			continue
		}
		// For symbols, check if they're in the allowed symbols whitelist
		if !slices.Contains(JSONKeyAllowedSymbols, string(char)) {
			return true
		}
	}

	return false
}

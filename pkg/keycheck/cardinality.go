package keycheck

import (
	"slices"
	"strings"
)

var (
	// The whitelist of allowed symbols only (no letters or numbers)
	JSONKeyAllowedSymbols   = []string{"_", ".", ":", "@", "-", "$", "#", "{", "}", "/"}
	BacktickRequiredSymbols = []string{":", "@", "-"}
)

func IsCardinal(key string) bool {
	length := len(key)

	// Very long keys are considered random by default
	if length > MaxKeyLength {
		return true
	}

	if hasNonAllowedSymbols(key) {
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

	return processKeySegments(key)
}

func IsBacktickRequired(key string) bool {
	return strings.ContainsAny(key, strings.Join(BacktickRequiredSymbols, ""))
}

func CleanBackticks(key string) string {
	return strings.ReplaceAll(key, "`", "")
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

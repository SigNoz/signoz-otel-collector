package keycheck

import (
	"regexp"
)

var (
	ipAddressRegex = regexp.MustCompile(`^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$`)
)

func IsCardinal(key string) bool {
	// whole string cases
	switch {
	case containsDigits(key):
		return true
	case hasNonAllowedSymbols(key):
		return true
	case ipAddressRegex.MatchString(key):
		return true
	}

	return processKeySegments(key)
}

// containsDigits checks if string contains digits
func containsDigits(s string) bool {
	if len(s) == 0 {
		return false
	}

	// Check if all characters are digits
	for _, char := range s {
		if char >= '0' && char <= '9' {
			return true
		}
	}
	return false
}

func hasNonAllowedSymbols(s string) bool {
	// Define the whitelist of allowed symbols only (no letters or numbers)
	allowedSymbols := map[rune]bool{
		'_': true, // underscore only for now
		'.': true, // dot only for now
	}

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
		if !allowedSymbols[char] {
			return true
		}
	}

	return false
}

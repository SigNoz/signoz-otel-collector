package utils

import "strings"

// Like implements ClickHouse-compatible LIKE pattern matching (case-sensitive).
//
// Pattern metacharacters:
//   - % matches any sequence of characters (including empty)
//   - _ matches any single Unicode character
//   - \% matches a literal '%'
//   - \_ matches a literal '_'
//   - \\ matches a literal '\'
//   - \x (any other x) matches a literal 'x' (backslash is consumed)
//
// https://clickhouse.com/docs/sql-reference/functions/string-search-functions#likes
func Like(s, pattern string) bool {
	return matchLike([]rune(s), []rune(pattern))
}

// ILike implements ClickHouse-compatible ILIKE pattern matching (case-insensitive).
// It is identical to Like but folds both the string and the pattern to lower-case
// before matching, matching ClickHouse's behaviour.
func ILike(s, pattern string) bool {
	return matchLike([]rune(strings.ToLower(s)), []rune(strings.ToLower(pattern)))
}

// matchLike is the recursive rune-level engine shared by Like and ILike.
func matchLike(s, pattern []rune) bool {
	for len(pattern) > 0 {
		switch pattern[0] {
		case '\\':
			if len(pattern) < 2 {
				// Trailing backslash — match it literally.
				if len(s) == 0 || s[0] != '\\' {
					return false
				}
				s = s[1:]
				pattern = pattern[1:]
			} else {
				next := pattern[1]
				// \%, \_, \\ are recognised escape sequences; anything else
				// is treated as a literal of the escaped character.
				if len(s) == 0 || s[0] != next {
					return false
				}
				s = s[1:]
				pattern = pattern[2:]
			}
		case '%':
			// Skip consecutive '%' wildcards.
			for len(pattern) > 0 && pattern[0] == '%' {
				pattern = pattern[1:]
			}
			if len(pattern) == 0 {
				return true
			}
			// Try anchoring the remaining pattern at every suffix of s.
			for i := 0; i <= len(s); i++ {
				if matchLike(s[i:], pattern) {
					return true
				}
			}
			return false
		case '_':
			if len(s) == 0 {
				return false
			}
			s = s[1:]
			pattern = pattern[1:]
		default:
			if len(s) == 0 || s[0] != pattern[0] {
				return false
			}
			s = s[1:]
			pattern = pattern[1:]
		}
	}
	return len(s) == 0
}

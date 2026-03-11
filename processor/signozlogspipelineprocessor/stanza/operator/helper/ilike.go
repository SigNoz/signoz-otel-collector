package signozstanzahelper

import (
	"regexp"
	"strings"
)

// compileILike compiles a LIKE pattern into a reusable case-insensitive matcher.
// Simple patterns use zero-allocation string operations (strings.EqualFold for
// exact/prefix/suffix; sliding EqualFold window for contains); everything else
// falls back to RE2 (?i).
//
// Note: the EqualFold-window approach compares byte-length slices. This is
// correct for ASCII and for Unicode code points whose upper/lower forms have
// the same UTF-8 byte length. For the rare cases where they differ (e.g. 'ß'
// ↔ "SS") the RE2 fallback handles them correctly via the kindRegexp path.
func compileILike(pattern string) (func(string) bool, error) {
	kind, lit1, lit2 := parseLikePattern(pattern)
	switch kind {
	case kindNoWildcards:
		return func(s string) bool { return strings.EqualFold(s, lit1) }, nil
	case kindPrefix:
		n := len(lit1)
		return func(s string) bool {
			return len(s) >= n && strings.EqualFold(s[:n], lit1)
		}, nil
	case kindSuffix:
		n := len(lit1)
		return func(s string) bool {
			return len(s) >= n && strings.EqualFold(s[len(s)-n:], lit1)
		}, nil
	case kindContains:
		n := len(lit1)
		return func(s string) bool {
			if len(s) < n {
				return false
			}
			for i := 0; i <= len(s)-n; i++ {
				if strings.EqualFold(s[i:i+n], lit1) {
					return true
				}
			}
			return false
		}, nil
	case kindPrefixSuffix:
		np, ns, min := len(lit1), len(lit2), len(lit1)+len(lit2)
		return func(s string) bool {
			return len(s) >= min &&
				strings.EqualFold(s[:np], lit1) &&
				strings.EqualFold(s[len(s)-ns:], lit2)
		}, nil
	default:
		re, err := regexp.Compile(`(?i)` + likePatternToRegexp(pattern))
		if err != nil {
			return nil, err
		}
		return re.MatchString, nil
	}
}

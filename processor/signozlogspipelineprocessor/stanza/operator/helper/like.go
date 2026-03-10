package signozstanzahelper

import (
	"fmt"
	"hash/fnv"
	"regexp"
	"strings"
)

// patternKind classifies a LIKE pattern into the cheapest matching tier.
type patternKind int

const (
	kindExact    patternKind = iota // no wildcards → s == literal
	kindPrefix                      // literal%     → strings.HasPrefix
	kindSuffix                      // %literal     → strings.HasSuffix
	kindContains                    // %literal%    → strings.Contains
	kindRegexp                      // everything else → RE2
)

// parseLikePattern analyses pattern and returns the cheapest tier plus the
// extracted literal (empty for kindRegexp). A pattern qualifies for a string
// tier only when it has no '_' wildcards and at most two unescaped '%' signs
// located exclusively at the very beginning and/or very end.
//
// Escape rules: \% → literal %, \_ → literal _, \\ → literal \, \x → literal x.
func parseLikePattern(pattern string) (kind patternKind, literal string) {
	runes := []rune(pattern)
	n := len(runes)

	leadingPct := n > 0 && runes[0] == '%'
	trailingPct := n > 0 && runes[n-1] == '%' && !(n > 1 && runes[n-2] == '\\')

	// Walk the pattern collecting the literal content and checking for
	// characters that force the RE2 fallback.
	var sb strings.Builder
	for i := 0; i < n; {
		ch := runes[i]
		switch {
		case ch == '%':
			// A '%' is allowed only as the first or last rune; anywhere else
			// we can't express it as a simple string op.
			if i != 0 && !(i == n-1 && trailingPct) {
				return kindRegexp, ""
			}
			i++
		case ch == '_':
			// Any unescaped '_' requires RE2.
			return kindRegexp, ""
		case ch == '\\' && i+1 < n:
			// Escape sequence: consume both runes, emit the escaped character.
			sb.WriteRune(runes[i+1])
			i += 2
		default:
			sb.WriteRune(ch)
			i++
		}
	}
	lit := sb.String()

	switch {
	case leadingPct && trailingPct:
		return kindContains, lit
	case leadingPct:
		return kindSuffix, lit
	case trailingPct:
		return kindPrefix, lit
	default:
		return kindExact, lit
	}
}

// compileLike compiles a LIKE pattern into a reusable case-sensitive matcher.
// Simple patterns (exact / prefix / suffix / contains) are handled with fast
// string operations; everything else falls back to RE2.
func compileLike(pattern string) (func(string) bool, error) {
	kind, lit := parseLikePattern(pattern)
	switch kind {
	case kindExact:
		return func(s string) bool { return s == lit }, nil
	case kindPrefix:
		return func(s string) bool { return strings.HasPrefix(s, lit) }, nil
	case kindSuffix:
		return func(s string) bool { return strings.HasSuffix(s, lit) }, nil
	case kindContains:
		return func(s string) bool { return strings.Contains(s, lit) }, nil
	default:
		re, err := regexp.Compile(likePatternToRegexp(pattern))
		if err != nil {
			return nil, err
		}
		return re.MatchString, nil
	}
}

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
	kind, lit := parseLikePattern(pattern)
	switch kind {
	case kindExact:
		return func(s string) bool { return strings.EqualFold(s, lit) }, nil
	case kindPrefix:
		n := len(lit)
		return func(s string) bool {
			if len(s) < n {
				return false
			}
			return strings.EqualFold(s[:n], lit)
		}, nil
	case kindSuffix:
		n := len(lit)
		return func(s string) bool {
			if len(s) < n {
				return false
			}
			return strings.EqualFold(s[len(s)-n:], lit)
		}, nil
	case kindContains:
		n := len(lit)
		return func(s string) bool {
			if len(s) < n {
				return false
			}
			for i := 0; i <= len(s)-n; i++ {
				if strings.EqualFold(s[i:i+n], lit) {
					return true
				}
			}
			return false
		}, nil
	default:
		re, err := regexp.Compile(`(?i)` + likePatternToRegexp(pattern))
		if err != nil {
			return nil, err
		}
		return re.MatchString, nil
	}
}

// likePatternToRegexp converts a LIKE pattern to a RE2-compatible regular
// expression string anchored at both ends. Called only for patterns that
// contain '_' or multiple '%' segments (the kindRegexp fallback path).
//
// Mapping:
//   - %  → .*  (any sequence; (?s) makes . match newlines too)
//   - _  → .   (any single Unicode character)
//   - \% → literal %
//   - \_ → literal _
//   - \\ → literal \
//   - \x → literal x (for any other x)
//   - other characters are passed through regexp.QuoteMeta
//
// https://clickhouse.com/docs/sql-reference/functions/string-search-functions#likes
func likePatternToRegexp(pattern string) string {
	var sb strings.Builder
	sb.WriteString(`(?s)^`)
	for i, runes := 0, []rune(pattern); i < len(runes); {
		ch := runes[i]
		switch ch {
		case '%':
			sb.WriteString(`.*`)
			i++
		case '_':
			sb.WriteByte('.')
			i++
		case '\\':
			if i+1 < len(runes) {
				sb.WriteString(regexp.QuoteMeta(string(runes[i+1])))
				i += 2
			} else {
				sb.WriteString(`\\`)
				i++
			}
		default:
			sb.WriteString(regexp.QuoteMeta(string(ch)))
			i++
		}
	}
	sb.WriteByte('$')
	return sb.String()
}

// LikeSlotName returns the env slot name used to inject a pre-compiled like
// matcher into the expr environment. The name is stable for a given pattern.
func likeSlotName(pattern string) string {
	return likeSlotNameF("like", pattern)
}

// ILikeSlotName returns the env slot name for a pre-compiled ilike matcher.
func iLikeSlotName(pattern string) string {
	return likeSlotNameF("ilike", pattern)
}

func likeSlotNameF(funcName, pattern string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(funcName))
	_, _ = h.Write([]byte{':'})
	_, _ = h.Write([]byte(pattern))
	return fmt.Sprintf("__%s_%08x", funcName, h.Sum32())
}

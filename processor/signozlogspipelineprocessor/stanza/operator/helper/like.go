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
	kindNoWildcards  patternKind = iota // no wildcards  → s == lit1
	kindPrefix                          // lit1%          → strings.HasPrefix(s, lit1)
	kindSuffix                          // %lit1          → strings.HasSuffix(s, lit1)
	kindContains                        // %lit1%         → strings.Contains(s, lit1)
	kindPrefixSuffix                    // lit1%lit2      → HasPrefix && HasSuffix
	kindRegexp                          // everything else → RE2
)

// parseLikePattern inspects pattern and returns the cheapest matching tier
// along with the one or two literal strings needed for matching.
//
// Supported fast tiers (no '_' wildcard, '%' only at boundaries):
//
//	literal     → kindNoWildcards,        lit1=literal
//	literal%    → kindPrefix,       lit1=literal
//	%literal    → kindSuffix,       lit1=literal
//	%literal%   → kindContains,     lit1=literal
//	lit1%lit2   → kindPrefixSuffix, lit1=prefix, lit2=suffix
//
// Any '_' wildcard or more than one interior '%' forces kindRegexp (lit1=""
// lit2=""), and the caller must fall back to RE2.
//
// Escape rules: \% → literal %, \_ → literal _, \\ → literal \, \x → literal x.
func parseLikePattern(pattern string) (kind patternKind, lit1, lit2 string) {
	runes := []rune(pattern)
	n := len(runes)

	leadingPct := n > 0 && runes[0] == '%'
	trailingPct := n > 0 && runes[n-1] == '%' && !(n > 1 && runes[n-2] == '\\')

	// Walk the pattern collecting literal characters. When we encounter an
	// unescaped '%' in the interior (not the leading/trailing sentinel), we
	// snapshot the first half and start collecting the second half. A second
	// interior '%' means we can't avoid RE2.
	var left, right strings.Builder
	cur := &left // writing into left until an interior '%' is seen
	middlePct := false

	for i := 0; i < n; {
		ch := runes[i]
		switch {
		case ch == '_':
			return kindRegexp, "", ""
		case ch == '\\' && i+1 < n:
			cur.WriteRune(runes[i+1])
			i += 2
		case ch == '%':
			isLeading := i == 0
			isTrailing := i == n-1 && trailingPct
			if isLeading || isTrailing {
				i++ // boundary sentinel — skip, don't emit
			} else if !middlePct {
				// First interior '%': everything written so far is the prefix;
				// switch cur to right so the rest becomes the suffix.
				middlePct = true
				cur = &right
				i++
			} else {
				// Second interior '%': too complex for string ops.
				return kindRegexp, "", ""
			}
		default:
			cur.WriteRune(ch)
			i++
		}
	}

	l, r := left.String(), right.String()

	switch {
	case middlePct:
		// Interior '%' only makes sense when there is no leading/trailing '%'
		// (e.g. `%a%b` or `a%b%` would mix tiers; fall back to RE2).
		if leadingPct || trailingPct {
			return kindRegexp, "", ""
		}
		return kindPrefixSuffix, l, r
	case leadingPct && trailingPct:
		return kindContains, l, ""
	case leadingPct:
		return kindSuffix, l, ""
	case trailingPct:
		return kindPrefix, l, ""
	default:
		return kindNoWildcards, l, ""
	}
}

// compileLike compiles a LIKE pattern into a reusable case-sensitive matcher.
// Simple patterns (exact / prefix / suffix / contains / prefix+suffix) are
// handled with fast string operations; everything else falls back to RE2.
func compileLike(pattern string) (func(string) bool, error) {
	kind, lit1, lit2 := parseLikePattern(pattern)
	switch kind {
	case kindNoWildcards:
		return func(s string) bool { return s == lit1 }, nil
	case kindPrefix:
		return func(s string) bool { return strings.HasPrefix(s, lit1) }, nil
	case kindSuffix:
		return func(s string) bool { return strings.HasSuffix(s, lit1) }, nil
	case kindContains:
		return func(s string) bool { return strings.Contains(s, lit1) }, nil
	case kindPrefixSuffix:
		min := len(lit1) + len(lit2)
		return func(s string) bool {
			return len(s) >= min && strings.HasPrefix(s, lit1) && strings.HasSuffix(s, lit2)
		}, nil
	default:
		re, err := regexp.Compile(likePatternToRegexp(pattern))
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

// likeSlotName returns the env slot name used to inject a pre-compiled like
// matcher into the expr environment. The name is stable for a given pattern.
func likeSlotName(pattern string) string {
	return likeSlotNameF("like", pattern)
}

func likeSlotNameF(funcName, pattern string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(funcName))
	_, _ = h.Write([]byte{':'})
	_, _ = h.Write([]byte(pattern))
	return fmt.Sprintf("__%s_%x", funcName, h.Sum64())
}

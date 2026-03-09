package signozstanzahelper

import (
	"fmt"
	"hash/fnv"
	"regexp"
	"strings"
)

// likePatternToRegexp converts a LIKE pattern to a RE2-compatible regular
// expression string anchored at both ends.
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

// CompileLike compiles a LIKE pattern into a reusable case-sensitive matcher.
// The returned function reports whether s matches the pattern.
func compileLike(pattern string) (func(string) bool, error) {
	re, err := regexp.Compile(likePatternToRegexp(pattern))
	if err != nil {
		return nil, err
	}
	return re.MatchString, nil
}

// CompileILike compiles a LIKE pattern into a reusable case-insensitive matcher.
func compileILike(pattern string) (func(string) bool, error) {
	re, err := regexp.Compile(`(?i)` + likePatternToRegexp(pattern))
	if err != nil {
		return nil, err
	}
	return re.MatchString, nil
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

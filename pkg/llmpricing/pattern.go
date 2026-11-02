package llmpricing

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// snapshotSuffix is what may follow a prefix: dated snapshots, "-latest",
// Bedrock "-vN:M" and Vertex "@" versions, in any combination.
var snapshotSuffix = regexp.MustCompile(`^(?:-\d{4}-\d{2}-\d{2}|-\d{2}-\d{4}|-\d{2}-\d{2}|-\d{3,8}|-latest|-v\d+(?::\d+)?|@\d{8}|@default)+$`)

// Pattern is a parsed model-name pattern: a case-sensitive literal prefix, and
// whether a trailing "*" lets it match any suffix.
type Pattern struct {
	prefix    string
	anySuffix bool
}

// ParsePattern splits a trailing "*" off the literal prefix.
func ParsePattern(s string) Pattern {
	prefix := strings.TrimSuffix(s, "*")
	return Pattern{prefix: prefix, anySuffix: prefix != s}
}

func (p Pattern) Prefix() string {
	return p.prefix
}

// Matches accepts the prefix itself or the prefix followed by snapshot suffixes
// only, so "claude-opus-5" never matches "claude-opus-5-5"; a trailing "*"
// accepts any suffix.
func (p Pattern) Matches(model string) bool {
	rest, ok := strings.CutPrefix(model, p.prefix)
	if !ok {
		return false
	}
	return rest == "" || p.anySuffix || snapshotSuffix.MatchString(rest)
}

// ValidatePattern rejects an empty pattern and a "*" anywhere but the end.
func ValidatePattern(s string) error {
	if s == "" {
		return errors.New("pattern must not be empty")
	}
	if strings.Contains(strings.TrimSuffix(s, "*"), "*") {
		return fmt.Errorf("pattern %q may only use \"*\" at the end", s)
	}
	return nil
}

package utils

import (
	"strings"
)

// BuildPromotedPaths extracts the given promoted paths from source into a new map, leaving source untouched.
// Values are shared with source, not copied. It returns nil when no path is found.
func BuildPromotedPaths(source map[string]any, promotedPaths map[string]struct{}) map[string]any {
	var promoted map[string]any
	for path := range promotedPaths {
		v, ok := extractPromotedPath(source, path)
		if !ok {
			continue
		}
		if promoted == nil {
			promoted = make(map[string]any, len(promotedPaths))
		}
		promoted[path] = v
	}
	return promoted
}

// extractPromotedPath walks source by remainingPath and returns the value,
// preferring a literal key match at each level before descending on '.'.
func extractPromotedPath(source map[string]any, remainingPath string) (any, bool) {
	if v, ok := source[remainingPath]; ok {
		if _, isMap := v.(map[string]any); !isMap { // ignore map values for extraction
			return v, true
		}
	}

	head, tail, ok := strings.Cut(remainingPath, ".")
	if !ok { // no nested path to check
		return nil, false
	}
	if nested, ok := source[head].(map[string]any); ok { // if value is not map, that means full path doesn't exist
		return extractPromotedPath(nested, tail)
	}
	return nil, false
}

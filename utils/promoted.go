package utils

import (
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// BuildPromotedPaths extracts the given promoted paths from source into a new map, leaving source untouched.
// It returns nil when no path is found.
func BuildPromotedPaths(source pcommon.Map, promotedPaths map[string]struct{}) map[string]any {
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

// extractPromotedPath walks source by remainingPath and returns the raw value,
// preferring a literal key match at each level before descending on '.'.
func extractPromotedPath(source pcommon.Map, remainingPath string) (any, bool) {
	if v, ok := source.Get(remainingPath); ok {
		if v.Type() != pcommon.ValueTypeMap { // ignore map values for extraction
			return v.AsRaw(), true
		}
	}

	head, tail, ok := strings.Cut(remainingPath, ".")
	if !ok { // no nested path to check
		return nil, false
	}
	if v, ok := source.Get(head); ok && v.Type() == pcommon.ValueTypeMap { // if value is not map, that means full path doesn't exist
		return extractPromotedPath(v.Map(), tail)
	}
	return nil, false
}

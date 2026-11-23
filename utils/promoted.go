package utils

import (
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// BuildPromotedPaths extracts the given promoted paths from source into a new map, leaving source untouched.
func BuildPromotedPaths(source pcommon.Map, promotedPaths map[string]struct{}) pcommon.Value {
	promoted := pcommon.NewValueMap()
	if len(promotedPaths) == 0 {
		return promoted
	}

	pm := promoted.Map()
	for path := range promotedPaths {
		extractPromotedPath(source, pm, path, path)
	}
	return promoted
}

// BuildPromotedPathsRaw is BuildPromotedPaths over a raw map; it returns nil when no path matches.
func BuildPromotedPathsRaw(source map[string]any, promotedPaths map[string]struct{}) map[string]any {
	var promoted map[string]any
	for path := range promotedPaths {
		if v, ok := extractPromotedPathRaw(source, path); ok {
			if promoted == nil {
				promoted = make(map[string]any, len(promotedPaths))
			}
			promoted[path] = v
		}
	}
	return promoted
}

func extractPromotedPathRaw(source map[string]any, remainingPath string) (any, bool) {
	if v, ok := source[remainingPath]; ok {
		if _, isMap := v.(map[string]any); !isMap {
			return v, true
		}
	}

	head, tail, ok := strings.Cut(remainingPath, ".")
	if !ok {
		return nil, false
	}
	if nested, ok := source[head].(map[string]any); ok {
		return extractPromotedPathRaw(nested, tail)
	}
	return nil, false
}

// extractPromotedPath walks source by remainingPath and copies the value into promotedMap
// at fullPath, preferring a literal key match at each level before descending on '.'.
func extractPromotedPath(source pcommon.Map, promotedMap pcommon.Map, fullPath, remainingPath string) {
	if v, ok := source.Get(remainingPath); ok {
		if v.Type() != pcommon.ValueTypeMap { // ignore map values for extraction
			v.CopyTo(promotedMap.PutEmpty(fullPath))
			return
		}
	}

	head, tail, ok := strings.Cut(remainingPath, ".")
	if !ok { // no nested path to check
		return
	}
	if v, ok := source.Get(head); ok && v.Type() == pcommon.ValueTypeMap { // if value is not map, that means full path doesn't exist
		extractPromotedPath(v.Map(), promotedMap, fullPath, tail)
	}
}

package utils

import (
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// BuildPromotedPaths extracts the given promoted paths from source into a new map,
// leaving source untouched. It is returned as a pcommon.Value so callers can either
// stringify it or take its .Map(). Shared by the traces and logs exporters.
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
	if !ok {
		return
	}
	if v, ok := source.Get(head); ok && v.Type() == pcommon.ValueTypeMap {
		extractPromotedPath(v.Map(), promotedMap, fullPath, tail)
	}
}

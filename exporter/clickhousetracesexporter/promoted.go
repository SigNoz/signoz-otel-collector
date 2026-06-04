package clickhousetracesexporter

import (
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// buildAttributesPromoted extracts promoted paths from span attributes into a
// map. The caller stringifies it via getAttributesJSON, same as the attributes column.
func buildAttributesPromoted(attrs pcommon.Map, promotedPaths []string) pcommon.Map {
	promoted := pcommon.NewValueMap()
	pm := promoted.Map()
	if attrs.Len() == 0 || len(promotedPaths) == 0 {
		return pm
	}

	for _, path := range promotedPaths {
		handleSingleAttributePath(attrs, pm, path, path)
	}
	return pm
}

// handleSingleAttributePath walks attrs according to remainingPath and extracts the value into promotedMap at fullPath.
func handleSingleAttributePath(bodyMap pcommon.Map, promotedMap pcommon.Map, fullPath string, remainingPath string) {
	if v, ok := bodyMap.Get(remainingPath); ok {
		if v.Type() != pcommon.ValueTypeMap { // ignore the map values for extraction
			dst := promotedMap.PutEmpty(fullPath)
			v.CopyTo(dst)
			return
		}
	}

	head, tail, ok := strings.Cut(remainingPath, ".")
	if !ok { // no nested path to check
		return
	}

	if v, ok := bodyMap.Get(head); ok && v.Type() == pcommon.ValueTypeMap { // if value is not map, that means full path doesn't exist
		handleSingleAttributePath(v.Map(), promotedMap, fullPath, tail)
	}
}

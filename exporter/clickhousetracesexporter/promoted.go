package clickhousetracesexporter

import (
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// buildAttributesPromoted extracts promoted paths from span attributes.
func buildAttributesPromoted(attrs pcommon.Map, promotedPaths []string) map[string]any {
	if attrs.Len() == 0 || len(promotedPaths) == 0 {
		return nil
	}

	promoted := pcommon.NewValueMap()
	pm := promoted.Map()
	for _, path := range promotedPaths {
		handleSingleAttributePath(attrs, pm, path, path)
	}

	if pm.Len() == 0 {
		return nil
	}
	return pm.AsRaw()
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

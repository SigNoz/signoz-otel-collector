package clickhouselogsexporter

import (
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// buildPromotedAndPruneBody extracts promoted paths from body and returns them as a separate map.
// It mutates the body in place to remove extracted entries.
func buildPromotedAndPruneBody(body pcommon.Value, promotedPaths map[string]struct{}) pcommon.Value {
	promoted := pcommon.NewValueMap()
	if body.Type() != pcommon.ValueTypeMap || len(promotedPaths) == 0 {
		return promoted
	}

	bm := body.Map()
	pm := promoted.Map()
	for path := range promotedPaths {
		// For each path, extract with literal preference at every level
		extractSinglePath(bm, pm, path, path)
	}

	// After extraction, group remaining dotted literal siblings into nested structures
	groupDottedLiterals(bm)

	// Clean up empty maps that were created during extraction
	cleanupEmptyMaps(bm)

	return promoted
}

// extractSinglePath walks the map according to remainingPath and extracts the value into promotedMap at fullPath.
// Literal preference: at the current map level, if a literal key equal to remainingPath exists, use it and stop descending.
func extractSinglePath(bodyMap pcommon.Map, promotedMap pcommon.Map, fullPath string, remainingPath string) {
	// Prefer literal match of the entire remaining path at this level
	if v, ok := bodyMap.Get(remainingPath); ok {
		dst := promotedMap.PutEmpty(fullPath)
		v.CopyTo(dst)
		bodyMap.Remove(remainingPath)
		return
	}

	// If no dot remains, try direct segment
	dot := strings.IndexByte(remainingPath, '.')
	if dot == -1 {
		if v, ok := bodyMap.Get(remainingPath); ok {
			dst := promotedMap.PutEmpty(fullPath)
			v.CopyTo(dst)
			bodyMap.Remove(remainingPath)
		}
		return
	}

	head := remainingPath[:dot]
	tail := remainingPath[dot+1:]

	// If there is no map under head, but there are dotted literal siblings like head.xxx,
	// materialize a submap under head and move those keys into it for consistent traversal.
	if v, ok := bodyMap.Get(head); !ok || v.Type() != pcommon.ValueTypeMap {
		// scan for dotted literals
		var foundPrefixed bool
		bodyMap.Range(func(k string, vv pcommon.Value) bool {
			if strings.HasPrefix(k, head+".") {
				foundPrefixed = true
				return false
			}
			return true
		})
		if foundPrefixed {
			// create submap and move entries
			sub := bodyMap.PutEmptyMap(head)
			var keysToMove []string
			bodyMap.Range(func(k string, vv pcommon.Value) bool {
				if strings.HasPrefix(k, head+".") {
					keysToMove = append(keysToMove, k)
				}
				return true
			})
			for _, k := range keysToMove {
				vv, _ := bodyMap.Get(k)
				// move only one level: head.rest -> sub[rest]
				rest := k[len(head)+1:]
				dst := sub.PutEmpty(rest)
				vv.CopyTo(dst)
				bodyMap.Remove(k)
			}
		}
	}

	if v, ok := bodyMap.Get(head); ok && v.Type() == pcommon.ValueTypeMap {
		// Recurse into nested map
		extractSinglePath(v.Map(), promotedMap, fullPath, tail)
	}
}

// groupDottedLiterals groups remaining dotted literal keys into nested structures
func groupDottedLiterals(bodyMap pcommon.Map) {
	// First, recursively process nested maps
	bodyMap.Range(func(k string, v pcommon.Value) bool {
		if v.Type() == pcommon.ValueTypeMap {
			groupDottedLiterals(v.Map())
		}
		return true
	})

	// Find all dotted literal keys and group them by prefix
	prefixGroups := make(map[string][]string)
	var keysToRemove []string

	bodyMap.Range(func(k string, v pcommon.Value) bool {
		if dot := strings.IndexByte(k, '.'); dot != -1 {
			prefix := k[:dot]
			prefixGroups[prefix] = append(prefixGroups[prefix], k)
		}
		return true
	})

	// For each prefix group, create a nested map and move the keys
	for prefix, keys := range prefixGroups {
		if len(keys) == 0 {
			continue
		}

		// Check if there's already a map at this prefix
		if existing, ok := bodyMap.Get(prefix); ok && existing.Type() == pcommon.ValueTypeMap {
			// Merge into existing map
			for _, k := range keys {
				if v, ok := bodyMap.Get(k); ok {
					suffix := k[len(prefix)+1:]
					dst := existing.Map().PutEmpty(suffix)
					v.CopyTo(dst)
					keysToRemove = append(keysToRemove, k)
				}
			}
		} else {
			// Create new nested map
			nested := bodyMap.PutEmptyMap(prefix)
			for _, k := range keys {
				if v, ok := bodyMap.Get(k); ok {
					suffix := k[len(prefix)+1:]
					dst := nested.PutEmpty(suffix)
					v.CopyTo(dst)
					keysToRemove = append(keysToRemove, k)
				}
			}
		}
	}

	// Remove the original dotted keys
	for _, k := range keysToRemove {
		bodyMap.Remove(k)
	}
}

// cleanupEmptyMaps removes empty maps from the body
func cleanupEmptyMaps(bodyMap pcommon.Map) {
	// First, recursively clean up nested maps
	bodyMap.Range(func(k string, v pcommon.Value) bool {
		if v.Type() == pcommon.ValueTypeMap {
			cleanupEmptyMaps(v.Map())
		}
		return true
	})

	// Then remove empty maps at this level
	var keysToRemove []string
	bodyMap.Range(func(k string, v pcommon.Value) bool {
		if v.Type() == pcommon.ValueTypeMap && v.Map().Len() == 0 {
			keysToRemove = append(keysToRemove, k)
		}
		return true
	})

	for _, k := range keysToRemove {
		bodyMap.Remove(k)
	}
}

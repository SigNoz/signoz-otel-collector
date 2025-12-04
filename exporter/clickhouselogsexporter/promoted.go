package clickhouselogsexporter

import (
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// buildPromotedAndPruneBody extracts promoted paths from body and returns them as a separate map.
// It mutates the body in place to remove extracted entries.
//
// Example:
//
//	Input body: {"message": "log", "user": {"id": "123", "name": "john"}}
//	Promoted paths: {"user.id"}
//	Result:
//	  - Promoted: {"user.id": "123"}
//	  - Body (mutated): {"message": "log", "user": {"name": "john"}}
//
// If all entries in a nested map are extracted, the empty map is automatically removed.
// Example: If "user.id" is the only key in "user", then "user" map is removed entirely.
func buildPromotedAndPruneBody(body pcommon.Value, promotedPaths map[string]struct{}) pcommon.Value {
	promoted := pcommon.NewValueMap()
	if body.Type() != pcommon.ValueTypeMap || len(promotedPaths) == 0 {
		return promoted
	}

	bm := body.Map()
	pm := promoted.Map()
	for path := range promotedPaths {
		// For each path, extract with literal preference at every level
		handleSinglePath(bm, pm, path, path)
	}

	return promoted
}

// handleSinglePath walks the map according to remainingPath and extracts the value into promotedMap at fullPath.
// It uses literal preference: at each map level, if a literal key matching the entire remaining path exists,
// it uses that literal key and stops descending. Otherwise, it splits on '.' and descends into nested maps.
//
// Literal preference example:
//
//	Body: {"a.b.c": "literal", "a": {"b": {"c": "nested"}}}
//	Path: "a.b.c"
//	Result: Extracts "a.b.c" (literal), not "a"."b"."c" (nested)
//
// The function also cleans up empty maps during the same iteration cycle - if a nested map becomes
// empty after extraction, it is removed immediately without requiring a separate pass.
//
// Parameters:
//   - bodyMap: The map to extract from (mutated in place)
//   - promotedMap: The map to store extracted values (keyed by fullPath)
//   - fullPath: The complete path for storing in promotedMap (e.g., "user.id")
//   - remainingPath: The remaining path to traverse (e.g., "user.id" or "id" when recursing)
func handleSinglePath(bodyMap pcommon.Map, promotedMap pcommon.Map, fullPath string, remainingPath string) {
	// Step 1: Prefer literal match of the entire remaining path at this level
	// Example: If remainingPath is "a.b.c" and bodyMap has key "a.b.c", extract it directly
	if v, ok := bodyMap.Get(remainingPath); ok {
		// we've found the path, but it's nested map, so we don't need to extract it
		if v.Type() != pcommon.ValueTypeMap {
			dst := promotedMap.PutEmpty(fullPath)
			v.CopyTo(dst)
			bodyMap.Remove(remainingPath)
			// Note: We can't remove bodyMap itself here, only its parent can do that after recursion
			// the code is written in a way that the parent will remove the bodyMap itself after recursion
			// check the end of the function for the cleanup
			return
		}
	}

	// Step 2: Split the path into head (first segment) and tail (remaining segments)
	// Example: "user.id" -> head="user", tail="id"
	indexOfNextDot := strings.IndexByte(remainingPath, '.')
	if indexOfNextDot == -1 {
		return
	}
	head := remainingPath[:indexOfNextDot]
	tail := remainingPath[indexOfNextDot+1:]

	// Step 3: Recurse into nested map if it exists
	// Example: bodyMap["user"] exists and is a map, recurse with tail="id"
	if v, ok := bodyMap.Get(head); ok && v.Type() == pcommon.ValueTypeMap {
		nestedMap := v.Map()
		handleSinglePath(nestedMap, promotedMap, fullPath, tail)
		// After recursion, check if the nested map is now empty and remove it immediately
		// This cleanup happens in the same iteration cycle, avoiding a separate pass
		// Example: If "user" map becomes empty after extracting "user.id", remove "user" key
		if nestedMap.Len() == 0 {
			bodyMap.Remove(head)
		}
	}
}

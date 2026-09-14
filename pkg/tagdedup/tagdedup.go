// Package tagdedup provides per-batch deduplication of rows written to the
// ClickHouse tag tables (attribute keys and attribute values), so that
// identical rows are written at most once per batch.
//
// The logic was originally introduced for spans in
// https://github.com/SigNoz/signoz-otel-collector/pull/177 and is shared
// between the traces and logs exporters.
package tagdedup

import (
	"strconv"
	"strings"

	"github.com/SigNoz/signoz-otel-collector/utils"
)

// separator is used to delimit components of a dedup ID. It is a unit
// separator so that components containing ':' or other printable characters
// cannot produce colliding IDs.
const separator = "\x1f"

// KeyID builds the dedup ID for an attribute key row identified by
// (key, tagType, dataType, isColumn).
func KeyID(key string, tagType utils.TagType, dataType utils.FieldDataType, isColumn bool) string {
	var id strings.Builder
	id.WriteString(key)
	id.WriteString(separator)
	id.WriteString(string(tagType))
	id.WriteString(separator)
	id.WriteString(string(dataType))
	id.WriteString(separator)
	id.WriteString(strconv.FormatBool(isColumn))
	return id.String()
}

// ValueID builds the dedup ID for an attribute value row identified by
// (key, tagType, dataType, stringValue, numberValue).
func ValueID(key string, tagType utils.TagType, dataType utils.FieldDataType, stringValue string, numberValue float64) string {
	var id strings.Builder
	id.WriteString(key)
	id.WriteString(separator)
	id.WriteString(string(tagType))
	id.WriteString(separator)
	id.WriteString(string(dataType))
	id.WriteString(separator)
	id.WriteString(stringValue)
	id.WriteString(separator)
	id.WriteString(strconv.FormatFloat(numberValue, 'f', -1, 64))
	return id.String()
}

// Deduper tracks attribute key and value rows already seen within a single
// batch. It is not safe for concurrent use; create one per batch.
type Deduper struct {
	keys   map[string]struct{}
	values map[string]struct{}
}

// New returns a Deduper ready for a new batch.
func New() *Deduper {
	return &Deduper{
		keys:   make(map[string]struct{}),
		values: make(map[string]struct{}),
	}
}

// SeenKeyID reports whether an attribute key row with the given ID (built with
// KeyID) has already been recorded in this batch. The first call for a new ID
// returns false and records it; subsequent calls return true.
func (d *Deduper) SeenKeyID(id string) bool {
	if _, ok := d.keys[id]; ok {
		return true
	}
	d.keys[id] = struct{}{}
	return false
}

// SeenValueID reports whether an attribute value row with the given ID (built
// with ValueID) has already been recorded in this batch. The first call for a
// new ID returns false and records it; subsequent calls return true.
func (d *Deduper) SeenValueID(id string) bool {
	if _, ok := d.values[id]; ok {
		return true
	}
	d.values[id] = struct{}{}
	return false
}

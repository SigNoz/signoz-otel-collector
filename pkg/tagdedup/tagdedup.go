// Package tagdedup provides per-batch deduplication of rows written to the
// ClickHouse tag tables, so identical rows are written at most once per batch.
package tagdedup

import (
	"strconv"
	"strings"

	"github.com/SigNoz/signoz-otel-collector/utils"
)

// separator is a unit separator so components containing ':' or other
// printable characters cannot produce colliding IDs.
const separator = "\x1f"

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

// Deduper is not safe for concurrent use; create one per batch.
type Deduper struct {
	keys   map[string]struct{}
	values map[string]struct{}
}

func New() *Deduper {
	return &Deduper{
		keys:   make(map[string]struct{}),
		values: make(map[string]struct{}),
	}
}

// SeenKeyID records id and reports whether it was already recorded.
func (d *Deduper) SeenKeyID(id string) bool {
	if _, ok := d.keys[id]; ok {
		return true
	}
	d.keys[id] = struct{}{}
	return false
}

// SeenValueID records id and reports whether it was already recorded.
func (d *Deduper) SeenValueID(id string) bool {
	if _, ok := d.values[id]; ok {
		return true
	}
	d.values[id] = struct{}{}
	return false
}

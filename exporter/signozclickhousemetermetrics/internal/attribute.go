package internal

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
)

type Value struct {
	DataType pcommon.ValueType
	Val      string
}

type Attributes map[string]Value

func NewAttributesFromPcommonMap(attrs pcommon.Map) Attributes {
	attrMap := make(map[string]Value, attrs.Len())
	attrs.Range(func(k string, v pcommon.Value) bool {
		attrMap[k] = Value{
			DataType: v.Type(),
			Val:      v.AsString(),
		}
		return true
	})

	return attrMap
}

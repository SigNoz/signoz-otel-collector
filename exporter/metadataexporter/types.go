package metadataexporter

import (
	"sync"

	"github.com/SigNoz/signoz-otel-collector/utils"
)

// Type name constants written to distributed_json_path_types.
const (
	typeString       = "String"
	typeInt64        = "Int64"
	typeFloat64      = "Float64"
	typeBool         = "Bool"
	typeArrayString  = "Array(Nullable(String))"
	typeArrayInt64   = "Array(Nullable(Int64))"
	typeArrayFloat64 = "Array(Nullable(Float64))"
	typeArrayBool    = "Array(Nullable(Bool))"
	typeArrayDynamic = "Array(Dynamic)"
	typeArrayJSON    = "Array(JSON)"
)

// mapArrayTypesToTagDataType excludes Array(JSON) type
var mapArrayTypesToTagDataType = map[string]utils.TagDataType{
	typeArrayString:  utils.TagDataTypeStringArray,
	typeArrayInt64:   utils.TagDataTypeNumberArray,
	typeArrayFloat64: utils.TagDataTypeNumberArray,
	typeArrayBool:    utils.TagDataTypeBoolArray,
	typeArrayDynamic: utils.TagDataTypeDynamicArray,
}

// bitmasks for compact per-leaf type aggregation across a batch.
const (
	maskString       uint16 = 1 << 0
	maskInt          uint16 = 1 << 1
	maskFloat        uint16 = 1 << 2
	maskBool         uint16 = 1 << 3
	maskArrayDynamic uint16 = 1 << 4
	maskArrayBool    uint16 = 1 << 5
	maskArrayFloat   uint16 = 1 << 6
	maskArrayInt     uint16 = 1 << 7
	maskArrayString  uint16 = 1 << 8
	maskArrayJSON    uint16 = 1 << 9
)

func maskToType(mask uint16) string {
	if mask&maskString != 0 {
		return typeString
	}
	if mask&maskInt != 0 {
		return typeInt64
	}
	if mask&maskFloat != 0 {
		return typeFloat64
	}
	if mask&maskBool != 0 {
		return typeBool
	}
	if mask&maskArrayString != 0 {
		return typeArrayString
	}
	if mask&maskArrayInt != 0 {
		return typeArrayInt64
	}
	if mask&maskArrayFloat != 0 {
		return typeArrayFloat64
	}
	if mask&maskArrayBool != 0 {
		return typeArrayBool
	}
	if mask&maskArrayJSON != 0 {
		return typeArrayJSON
	}
	if mask&maskArrayDynamic != 0 {
		return typeArrayDynamic
	}
	return ""
}

// typeSet is a per-batch accumulator mapping JSON paths to their observed ClickHouse types.
// sync.Map is used because the jsonProcessor walk may be called from concurrent contexts
// in the future; it is safe to use sequentially too.
type typeSet struct {
	types sync.Map // map[string]*utils.ConcurrentSet[string]
}

func (t *typeSet) record(path string, mask uint16) {
	actual, _ := t.types.LoadOrStore(path, utils.WithCapacityConcurrentSet[string](3))
	cs := actual.(*utils.ConcurrentSet[string])

	cs.Insert(maskToType(mask))
}

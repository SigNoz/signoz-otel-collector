package metadataexporter

import (
	"sync"

	"github.com/SigNoz/signoz-otel-collector/utils"
)

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

func maskToType(mask uint16) utils.FieldDataType {
	switch {
	case mask&maskString != 0:
		return utils.FieldDataTypeString
	case mask&maskInt != 0:
		return utils.FieldDataTypeInt64
	case mask&maskFloat != 0:
		return utils.FieldDataTypeFloat64
	case mask&maskBool != 0:
		return utils.FieldDataTypeBool
	case mask&maskArrayString != 0:
		return utils.FieldDataTypeArrayString
	case mask&maskArrayInt != 0:
		return utils.FieldDataTypeArrayInt64
	case mask&maskArrayFloat != 0:
		return utils.FieldDataTypeArrayFloat64
	case mask&maskArrayBool != 0:
		return utils.FieldDataTypeArrayBool
	case mask&maskArrayJSON != 0:
		return utils.FieldDataTypeArrayJSON
	case mask&maskArrayDynamic != 0:
		return utils.FieldDataTypeArrayDynamic
	default:
		return ""
	}
}

type typesConcurrentSet = utils.ConcurrentSet[utils.FieldDataType]

// typesAccumulator is a per-batch accumulator mapping JSON paths to their observed ClickHouse types.
// sync.Map is used because the jsonProcessor walk may be called from concurrent contexts
// in the future; it is safe to use sequentially too.
type typesAccumulator struct {
	types sync.Map // map[string]*utils.ConcurrentSet[utils.FieldDataType]
}

func (t *typesAccumulator) record(path string, mask uint16) {
	actual, _ := t.types.LoadOrStore(path, utils.WithCapacityConcurrentSet[utils.FieldDataType](3))
	cs := actual.(*typesConcurrentSet)

	cs.Insert(maskToType(mask))
}

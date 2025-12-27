package jsontypeexporter

import (
	"sync"

	"github.com/SigNoz/signoz-otel-collector/utils"
)

const (
	String       = "String"
	Int64        = "Int64"
	Float64      = "Float64"
	Bool         = "Bool"
	ArrayString  = "Array(Nullable(String))"
	ArrayInt64   = "Array(Nullable(Int64))"
	ArrayFloat64 = "Array(Nullable(Float64))"
	ArrayBool    = "Array(Nullable(Bool))"
	ArrayDynamic = "Array(Dynamic)"
	ArrayJSON    = "Array(JSON)"
)

// bitmasks for compact aggregation
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

type TypeSet struct {
	types sync.Map // map[string]*utils.ConcurrentSet[string]
}

func (t *TypeSet) Insert(path string, mask uint16) {
	actual, _ := t.types.LoadOrStore(path, utils.WithCapacityConcurrentSet[string](3))
	cs := actual.(*utils.ConcurrentSet[string])
	// expand mask to strings
	if mask&maskString != 0 {
		cs.Insert(String)
	}
	if mask&maskInt != 0 {
		cs.Insert(Int64)
	}
	if mask&maskFloat != 0 {
		cs.Insert(Float64)
	}
	if mask&maskBool != 0 {
		cs.Insert(Bool)
	}
	if mask&maskArrayString != 0 {
		cs.Insert(ArrayString)
	}
	if mask&maskArrayInt != 0 {
		cs.Insert(ArrayInt64)
	}
	if mask&maskArrayFloat != 0 {
		cs.Insert(ArrayFloat64)
	}
	if mask&maskArrayBool != 0 {
		cs.Insert(ArrayBool)
	}
	if mask&maskArrayJSON != 0 {
		cs.Insert(ArrayJSON)
	}
	if mask&maskArrayDynamic != 0 {
		cs.Insert(ArrayDynamic)
	}
}

package jsontypeexporter

import (
	"sync"

	"github.com/SigNoz/signoz-otel-collector/utils"
	lru "github.com/hashicorp/golang-lru/v2"
)

const (
	StringType   = "String"
	IntType      = "Int"
	Float64Type  = "Float64"
	BooleanType  = "Bool"
	ArrayDynamic = "Array(Dynamic)"
	ArrayBoolean = "Array(Nullable(Bool))"
	ArrayFloat64 = "Array(Nullable(Float64))"
	ArrayInt     = "Array(Nullable(Int))"
	ArrayString  = "Array(Nullable(String))"
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
	pathCache *lru.Cache[string, *utils.ConcurrentSet[string]] // to skip adding duplicate entries for existing path and types combination
	types     sync.Map                                         // map[string]*utils.ConcurrentSet[string]
}

func NewTypeSet(pathCache *lru.Cache[string, *utils.ConcurrentSet[string]]) *TypeSet {
	return &TypeSet{
		pathCache: pathCache,
		types:     sync.Map{},
	}
}

func (t *TypeSet) Insert(path string, mask uint16) {
	actual, _ := t.types.LoadOrStore(path, utils.WithCapacityConcurrentSet[string](3))
	cs := actual.(*utils.ConcurrentSet[string])
	var typ string
	switch {
	case mask&maskString != 0:
		typ = StringType
	case mask&maskInt != 0:
		typ = IntType
	case mask&maskFloat != 0:
		typ = Float64Type
	case mask&maskBool != 0:
		typ = BooleanType
	case mask&maskArrayString != 0:
		typ = ArrayString
	case mask&maskArrayInt != 0:
		typ = ArrayInt
	case mask&maskArrayFloat != 0:
		typ = ArrayFloat64
	case mask&maskArrayBool != 0:
		typ = ArrayBoolean
	case mask&maskArrayJSON != 0:
		typ = ArrayJSON
	case mask&maskArrayDynamic != 0:
		typ = ArrayDynamic
	}

	set, ok := t.pathCache.Get(path)
	if !ok {
		set = utils.WithCapacityConcurrentSet[string](3)
		t.pathCache.Add(path, set)
	}

	// if not recorded yet, record it
	if !set.Contains(typ) {
		set.Insert(typ)
		cs.Insert(typ)
	}
}

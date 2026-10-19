package chjson

import (
	"encoding/base64"
	"math"
	"strconv"

	"github.com/ClickHouse/clickhouse-go/v2/lib/chcol"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

const DefaultPathBudgetPerBatch = 1000

func ExceedsPathBudget(obj *chcol.JSON, seen map[string]struct{}, budget int) bool {
	if len(seen) == 0 {
		return false
	}
	newPaths := 0
	for path := range obj.ValuesByPath() {
		if _, ok := seen[path]; !ok {
			newPaths++
		}
	}
	return len(seen)+newPaths > budget
}

func RecordPaths(obj *chcol.JSON, seen map[string]struct{}) {
	for path := range obj.ValuesByPath() {
		seen[path] = struct{}{}
	}
}

type Limits struct {
	MaxDynamicTypes int
	MaxDynamicPaths int
}

var DefaultLimits = Limits{MaxDynamicTypes: 32, MaxDynamicPaths: 1024}

// nested returns the limits ClickHouse assigns to objects inside an Array(JSON) path.
func (l Limits) nested() Limits {
	return Limits{MaxDynamicTypes: l.MaxDynamicTypes / 2, MaxDynamicPaths: l.MaxDynamicPaths / 4}
}

// FromPcommonMap stores every value with the type ClickHouse infers when parsing the map's JSON text,
// except that date-like strings stay String. Where that text fails to insert, the first duplicate
// path wins, integers beyond UInt64 stay Float64, and NaN/Inf become null.
func FromPcommonMap(m pcommon.Map, limits Limits, typedStringPaths map[string]struct{}) *chcol.JSON {
	obj := chcol.NewJSON()
	for k, v := range m.All() {
		if _, ok := typedStringPaths[k]; ok {
			setFirst(obj, k, v.AsString())
			continue
		}
		setPathValue(obj, k, v, limits)
	}
	return obj
}

// setFirst keeps the first non-null value of a path; ClickHouse treats null as an absent path.
func setFirst(obj *chcol.JSON, path string, value any) {
	if existing, exists := obj.ValueAtPath(path); exists && existing != nil {
		return
	}
	obj.SetValueAtPath(path, value)
}

func setPathValue(obj *chcol.JSON, path string, v pcommon.Value, limits Limits) {
	if v.Type() == pcommon.ValueTypeMap {
		for k, nv := range v.Map().All() {
			setPathValue(obj, path+"."+k, nv, limits)
		}
		return
	}
	if isNullValue(v) {
		setFirst(obj, path, nil)
		return
	}
	t := elementType(v)
	objectLimits := limits.nested()
	setFirst(obj, path, chcol.NewDynamicWithType(convert(v, t, &objectLimits), typeName(t, &objectLimits)))
}

func isNullValue(v pcommon.Value) bool {
	if v.Type() == pcommon.ValueTypeEmpty {
		return true
	}
	return v.Type() == pcommon.ValueTypeDouble && (math.IsNaN(v.Double()) || math.IsInf(v.Double(), 0))
}

// convert builds the driver value of v for type t; objectLimits is nil inside Dynamic, where objects
// are plain JSON with default limits.
func convert(v pcommon.Value, t *chType, objectLimits *Limits) any {
	if isNullValue(v) {
		switch removeNullable(t).id {
		case tArray:
			return []any{}
		case tObject:
			// An untyped nil as the first row switches the driver's JSON column to string mode.
			return (*chcol.JSON)(nil)
		}
		return nil
	}
	switch t.id {
	case tNullable:
		return convert(v, t.nested, objectLimits)
	case tBool:
		return v.Bool()
	case tInt64:
		if v.Type() == pcommon.ValueTypeInt {
			return v.Int()
		}
		return int64(numericValue(v))
	case tUInt64:
		if v.Type() == pcommon.ValueTypeInt {
			return uint64(v.Int())
		}
		return uint64(numericValue(v))
	case tFloat64:
		if v.Type() == pcommon.ValueTypeInt {
			return float64(v.Int())
		}
		return numericValue(v)
	case tObject:
		limits := DefaultLimits
		if objectLimits != nil {
			limits = *objectLimits
		}
		return FromPcommonMap(v.Map(), limits, nil)
	case tArray:
		elems := make([]any, 0, v.Slice().Len())
		for _, el := range v.Slice().All() {
			elems = append(elems, convert(el, t.nested, objectLimits))
		}
		return elems
	case tDynamic:
		et := elementType(v)
		return chcol.NewDynamicWithType(convert(v, et, nil), typeName(et, nil))
	default:
		return stringValue(v)
	}
}

func numericValue(v pcommon.Value) float64 {
	switch v.Type() {
	case pcommon.ValueTypeInt:
		return float64(v.Int())
	case pcommon.ValueTypeBool:
		if v.Bool() {
			return 1
		}
		return 0
	default:
		return v.Double()
	}
}

func stringValue(v pcommon.Value) string {
	switch v.Type() {
	case pcommon.ValueTypeBytes:
		return base64.StdEncoding.EncodeToString(v.Bytes().AsRaw())
	case pcommon.ValueTypeBool:
		return strconv.FormatBool(v.Bool())
	default:
		return v.Str()
	}
}

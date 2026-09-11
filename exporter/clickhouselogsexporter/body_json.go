package clickhouselogsexporter

import (
	"encoding/base64"

	"github.com/ClickHouse/clickhouse-go/v2/lib/chcol"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

var bodyV2TypedStringPaths = map[string]struct{}{
	bodyNonMapKey: {},
}

func pcommonMapToChJSON(m pcommon.Map, typedStringPaths map[string]struct{}) *chcol.JSON {
	obj := chcol.NewJSON()
	for k, v := range m.All() {
		if _, ok := typedStringPaths[k]; ok {
			obj.SetValueAtPath(k, v.AsString())
			continue
		}
		setPathValue(obj, k, v)
	}
	return obj
}

func setPathValue(obj *chcol.JSON, path string, v pcommon.Value) {
	switch v.Type() {
	case pcommon.ValueTypeMap:
		for k, nv := range v.Map().All() {
			setPathValue(obj, path+"."+k, nv)
		}
	case pcommon.ValueTypeSlice:
		obj.SetValueAtPath(path, sliceValue(v.Slice()))
	default:
		obj.SetValueAtPath(path, scalarValue(v))
	}
}

func scalarValue(v pcommon.Value) any {
	switch v.Type() {
	case pcommon.ValueTypeStr:
		return v.Str()
	case pcommon.ValueTypeInt:
		return v.Int()
	case pcommon.ValueTypeDouble:
		return v.Double()
	case pcommon.ValueTypeBool:
		return v.Bool()
	case pcommon.ValueTypeBytes:
		return base64.StdEncoding.EncodeToString(v.Bytes().AsRaw())
	default:
		return nil
	}
}

func sliceValue(s pcommon.Slice) chcol.Dynamic {
	kind, uniform := classifySlice(s)
	if uniform {
		switch kind {
		case pcommon.ValueTypeInt:
			return chcol.NewDynamicWithType(scalarElements(s), "Array(Nullable(Int64))")
		case pcommon.ValueTypeDouble:
			return chcol.NewDynamicWithType(scalarElements(s), "Array(Nullable(Float64))")
		case pcommon.ValueTypeBool:
			return chcol.NewDynamicWithType(scalarElements(s), "Array(Nullable(Bool))")
		case pcommon.ValueTypeStr, pcommon.ValueTypeEmpty:
			return chcol.NewDynamicWithType(scalarElements(s), "Array(Nullable(String))")
		case pcommon.ValueTypeMap:
			elems := make([]any, 0, s.Len())
			for _, el := range s.All() {
				elems = append(elems, pcommonMapToChJSON(el.Map(), nil))
			}
			return chcol.NewDynamicWithType(elems, "Array(JSON)")
		}
	}

	elems := make([]any, 0, s.Len())
	for _, el := range s.All() {
		switch el.Type() {
		case pcommon.ValueTypeMap:
			elems = append(elems, chcol.NewDynamicWithType(pcommonMapToChJSON(el.Map(), nil), "JSON"))
		case pcommon.ValueTypeSlice:
			elems = append(elems, sliceValue(el.Slice()))
		default:
			elems = append(elems, scalarValue(el))
		}
	}
	return chcol.NewDynamicWithType(elems, "Array(Dynamic)")
}

func classifySlice(s pcommon.Slice) (pcommon.ValueType, bool) {
	kind := pcommon.ValueTypeEmpty
	hasNull := false
	for _, el := range s.All() {
		t := el.Type()
		if t == pcommon.ValueTypeBytes {
			t = pcommon.ValueTypeStr
		}
		switch {
		case t == pcommon.ValueTypeEmpty:
			hasNull = true
		case t == pcommon.ValueTypeSlice:
			return 0, false
		case kind == pcommon.ValueTypeEmpty:
			kind = t
		case t != kind:
			return 0, false
		}
	}
	if kind == pcommon.ValueTypeMap && hasNull {
		return 0, false
	}
	return kind, true
}

func scalarElements(s pcommon.Slice) []any {
	elems := make([]any, 0, s.Len())
	for _, el := range s.All() {
		elems = append(elems, scalarValue(el))
	}
	return elems
}

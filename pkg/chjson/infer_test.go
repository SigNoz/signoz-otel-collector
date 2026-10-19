package chjson

import (
	"math"
	"sort"
	"strings"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2/lib/chcol"
	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func mapFromJSON(t *testing.T, doc string) pcommon.Map {
	t.Helper()
	var raw map[string]any
	require.NoError(t, sonic.Config{UseInt64: true}.Froze().UnmarshalFromString(doc, &raw))
	m := pcommon.NewMap()
	require.NoError(t, m.FromRaw(raw))
	return m
}

// typeSignature renders every path with its stored type, descending into arrays and values that hold
// JSON or Dynamic, e.g. {a:Array(Dynamic)[Dynamic<Int64>,null]}. Null paths are omitted.
func typeSignature(obj *chcol.JSON) string {
	byPath := obj.ValuesByPath()
	paths := make([]string, 0, len(byPath))
	for p, v := range byPath {
		if v != nil {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	parts := make([]string, len(paths))
	for i, p := range paths {
		switch v := byPath[p].(type) {
		case chcol.Dynamic:
			parts[i] = p + ":" + valueSignature(v.Type(), v.Any())
		default:
			parts[i] = p + ":String"
		}
	}
	return "{" + strings.Join(parts, " ") + "}"
}

func valueSignature(typ string, v any) string {
	if !strings.Contains(typ, "JSON") && !strings.Contains(typ, "Dynamic") {
		return typ
	}
	if inner, ok := strings.CutPrefix(typ, "Array("); ok {
		inner = strings.TrimSuffix(inner, ")")
		elems := v.([]any)
		parts := make([]string, len(elems))
		for i, el := range elems {
			parts[i] = valueSignature(inner, el)
		}
		return typ + "[" + strings.Join(parts, ",") + "]"
	}
	if inner, ok := strings.CutPrefix(typ, "Nullable("); ok {
		if obj, isJSON := v.(*chcol.JSON); v == nil || (isJSON && obj == nil) {
			return "null"
		}
		return valueSignature(strings.TrimSuffix(inner, ")"), v)
	}
	if typ == "Dynamic" {
		if v == nil {
			return "null"
		}
		d := v.(chcol.Dynamic)
		return "Dynamic<" + valueSignature(d.Type(), d.Any()) + ">"
	}
	return typeSignature(v.(*chcol.JSON))
}

func TestFromPcommonMapInferredTypes(t *testing.T) {
	testCases := []struct {
		name string
		doc  string
		fill func(m pcommon.Map)
		// textFails: ClickHouse rejects this body as JSON text; want is the typed insert's behavior.
		textFails bool
		// deviates: date-like strings stay String, as with date inference off on the exporter connection.
		deviates bool
		want     string
	}{
		{name: "Scalars", doc: `{"s":"a","i":1,"f":1.5,"b":true,"n":null,"e":""}`, want: "{b:Bool e:String f:Float64 i:Int64 s:String}"},
		{name: "IntBoundaries", doc: `{"max":9223372036854775807,"min":-9223372036854775808}`, want: "{max:Int64 min:Int64}"},
		{name: "IntegralDoubles", fill: func(m pcommon.Map) {
			m.PutDouble("whole", 200)
			m.PutDouble("neg_zero", math.Copysign(0, -1))
			m.PutDouble("above_int64", 9.3e18)
			m.PutDouble("e19", 1e19)
		}, want: "{above_int64:UInt64 e19:UInt64 neg_zero:Int64 whole:Int64}"},
		{name: "Floats", doc: `{"f":1e300,"g":1.5e21,"h":0.000001,"tiny":1e-7,"neg":-2.25}`, want: "{f:Float64 g:Float64 h:Float64 neg:Float64 tiny:Float64}"},
		{name: "IntegerBeyondUInt64_TextFails", doc: `{"u":18446744073709551615,"e20":100000000000000000000}`, textFails: true, want: "{e20:Float64 u:Float64}"},
		{name: "NaNAndInf_TextFails", fill: func(m pcommon.Map) {
			m.PutDouble("nan", math.NaN())
			m.PutDouble("inf", math.Inf(-1))
			m.PutInt("k", 1)
		}, textFails: true, want: "{k:Int64}"},
		{name: "Bytes", fill: func(m pcommon.Map) { m.PutEmptyBytes("b").FromRaw([]byte("hi")) }, want: "{b:String}"},
		{name: "DateLikeStrings_StayString", doc: `{"d":"2024-02-30","dt":"2024-01-01 25:00:00","slash":"2024/01/01","ms":"2024-01-01 10:00:00.123","iso":"2024-01-01T10:00:00Z"}`, deviates: true, want: "{d:String dt:String iso:String ms:String slash:String}"},
		{name: "NumberAndBoolLikeStrings", doc: `{"n":"123","f":"1.5","b":"true","u":"6f1e6c3a-3c1e-4bde-9a3e-0f8a7f2b9c11","ip":"10.0.0.1"}`, want: "{b:String f:String ip:String n:String u:String}"},
		{name: "OddKeys", doc: `{"":1,"sp ace":2,"quo\"te":3,"back` + "`" + `tick":4,"ünï":5}`, want: "{:Int64 back`tick:Int64 quo\"te:Int64 sp ace:Int64 ünï:Int64}"},
		{name: "DotEdgeKeys", doc: `{".lead":1,"trail.":2,"dou..ble":3}`, want: "{.lead:Int64 dou..ble:Int64 trail.:Int64}"},
		{name: "DuplicatePath_DottedFirst_TextFails", fill: func(m pcommon.Map) {
			m.PutStr("db.function", "x")
			m.PutEmptyMap("db").PutInt("function", 1)
		}, textFails: true, want: "{db.function:String}"},
		{name: "DuplicatePath_NestedFirst_TextFails", fill: func(m pcommon.Map) {
			m.PutEmptyMap("db").PutInt("function", 1)
			m.PutStr("db.function", "x")
		}, textFails: true, want: "{db.function:Int64}"},
		{name: "DuplicatePath_NullFirst_ValueKept", fill: func(m pcommon.Map) {
			m.PutEmpty("a.b")
			m.PutEmptyMap("a").PutInt("b", 3)
			m.PutEmptyMap("b").PutEmpty("c")
			m.PutBool("b.c", true)
		}, want: "{a.b:Int64 b.c:Bool}"},
		{name: "LeafAndChildPaths", fill: func(m pcommon.Map) {
			m.PutInt("a.b", 1)
			m.PutEmptyMap("a").PutEmptyMap("b").PutInt("c", 2)
		}, want: "{a.b:Int64 a.b.c:Int64}"},
		{name: "EmptyMaps", doc: `{"e":{},"n":{"e":{}},"k":1}`, want: "{k:Int64}"},
		{name: "DeepNesting", doc: `{"a":{"b":{"c":{"d":{"e":{"f":1}}}}}}`, want: "{a.b.c.d.e.f:Int64}"},
		{name: "ArrayEmptyAndNull", doc: `{"e":[],"n":[null,null]}`, want: "{e:Array(Nullable(String)) n:Array(Nullable(String))}"},
		{name: "ArrayScalarsUniform", doc: `{"s":["a",null],"i":[1,null],"f":[1.5],"b":[true,false]}`, want: "{b:Array(Nullable(Bool)) f:Array(Nullable(Float64)) i:Array(Nullable(Int64)) s:Array(Nullable(String))}"},
		{name: "ArrayScalarsMixed", doc: `{"if":[1,2.5],"ib":[1,true],"fb":[1.5,true],"si":["a",1],"sb":["a",true],"bs":[true,"a"],"ibf":[1,true,2.5],"sf":["x",1.5],"sbi":["x",true,1],"nif":[null,1,2.5]}`, want: "{bs:Array(Nullable(String)) fb:Array(Nullable(Float64)) ib:Array(Nullable(Int64)) ibf:Array(Nullable(Float64)) if:Array(Nullable(Float64)) nif:Array(Nullable(Float64)) sb:Array(Nullable(String)) sbi:Array(Dynamic)[Dynamic<String>,Dynamic<Bool>,Dynamic<Int64>] sf:Array(Dynamic)[Dynamic<String>,Dynamic<Float64>] si:Array(Dynamic)[Dynamic<String>,Dynamic<Int64>]}"},
		{name: "ArrayUInt64", doc: `{"iu":[1,10000000000000000000],"nu":[-1,10000000000000000000],"fu":[1.5,10000000000000000000],"bu":[true,10000000000000000000]}`, want: "{bu:Array(Nullable(UInt64)) fu:Array(Nullable(Float64)) iu:Array(Nullable(UInt64)) nu:Array(Dynamic)[Dynamic<Int64>,Dynamic<UInt64>]}"},
		{name: "ArrayDateStrings_StayString", doc: `{"d":["2024-01-01","2024-01-02"],"dx":["2024-01-01","x"],"dn":["2024-01-01",1]}`, deviates: true, want: "{d:Array(Nullable(String)) dn:Array(Dynamic)[Dynamic<String>,Dynamic<Int64>] dx:Array(Nullable(String))}"},
		{name: "ArrayObjects", doc: `{"o":[{"k":1},{"k":2,"j":"x"}],"c":[{"k":1},{"k":"1"},{"k":2.5}],"kv":[{"k":1},{"k":[1]},{"k":{"j":1}}]}`, want: "{c:Array(JSON(max_dynamic_types=16, max_dynamic_paths=0))[{k:Int64},{k:String},{k:Float64}] kv:Array(JSON(max_dynamic_types=16, max_dynamic_paths=0))[{k:Int64},{k:Array(Nullable(Int64))},{k.j:Int64}] o:Array(JSON(max_dynamic_types=16, max_dynamic_paths=0))[{k:Int64},{j:String k:Int64}]}"},
		{name: "ArrayObjectsWithNull", doc: `{"o":[{"k":1},null],"first":[null,{"k":1}],"empty":[{},null]}`, want: "{empty:Array(Nullable(JSON(max_dynamic_types=16, max_dynamic_paths=0)))[{},null] first:Array(Nullable(JSON(max_dynamic_types=16, max_dynamic_paths=0)))[null,{k:Int64}] o:Array(Nullable(JSON(max_dynamic_types=16, max_dynamic_paths=0)))[{k:Int64},null]}"},
		{name: "ArrayObjectsMixed", doc: `{"o":[{"k":1},"s",2,true,null],"oa":[{"k":1},[1]]}`, want: "{o:Array(Dynamic)[Dynamic<{k:Int64}>,Dynamic<String>,Dynamic<Int64>,Dynamic<Bool>,null] oa:Array(Dynamic)[Dynamic<{k:Int64}>,Dynamic<Array(Nullable(Int64))>]}"},
		{name: "ArrayObjectsNested", doc: `{"o":[{"k":1,"in":[{"z":3,"deep":[{"w":4}]}]}]}`, want: "{o:Array(JSON(max_dynamic_types=16, max_dynamic_paths=0))[{in:Array(JSON(max_dynamic_types=8, max_dynamic_paths=0))[{deep:Array(JSON(max_dynamic_types=4, max_dynamic_paths=0))[{w:Int64}] z:Int64}] k:Int64}]}"},
		{name: "ArrayObjectsElementArrays", doc: `{"o":[{"k":[1]},{"k":[]},{"k":[null]},{"k":["a",1]}]}`, want: "{o:Array(JSON(max_dynamic_types=16, max_dynamic_paths=0))[{k:Array(Nullable(Int64))},{k:Array(Nullable(String))},{k:Array(Nullable(String))},{k:Array(Dynamic)[Dynamic<String>,Dynamic<Int64>]}]}"},
		{name: "ArrayOfEmptyObjects", doc: `{"o":[{},{}],"m":[{},{"k":1}]}`, want: "{m:Array(JSON(max_dynamic_types=16, max_dynamic_paths=0))[{},{k:Int64}] o:Array(JSON(max_dynamic_types=16, max_dynamic_paths=0))[{},{}]}"},
		{name: "NestedArrays", doc: `{"ii":[[1],[2]],"if":[[1],[2.5]],"is":[[1],["a"]],"ie":[[],[1]],"in":[[null],[1]],"io":[[{"a":1}]],"ia":[1,[2]],"iii":[[[1]]],"ee":[[],[]],"nn":[[null]],"inull":[[1],null]}`, want: "{ee:Array(Array(Nullable(String))) ia:Array(Dynamic)[Dynamic<Int64>,Dynamic<Array(Nullable(Int64))>] ie:Array(Array(Nullable(Int64))) if:Array(Array(Nullable(Float64))) ii:Array(Array(Nullable(Int64))) iii:Array(Array(Array(Nullable(Int64)))) in:Array(Array(Nullable(Int64))) inull:Array(Array(Nullable(Int64))) io:Array(Array(JSON(max_dynamic_types=16, max_dynamic_paths=0)))[Array(JSON(max_dynamic_types=16, max_dynamic_paths=0))[{a:Int64}]] is:Array(Dynamic)[Dynamic<Array(Nullable(Int64))>,Dynamic<Array(Nullable(String))>] nn:Array(Array(Nullable(String)))}"},
		{name: "NestedArraysUnify", doc: `{"ib":[[1],[true]],"fb":[[1.5],[true]],"sb":[["x"],[true]],"dyn_int":[[1,"a"],[2]],"dyn_str":[[1,"a"],["b"]],"dyn_empty":[[1,"a"],[]],"deep_conflict":[[[1]],[["a"]]],"obj_null":[[{"k":1}],null],"obj_int":[[{"k":1}],[1]],"obj_deep":[[[{"k":1}]]],"obj_str_k":[[{"k":1}],[{"k":"x"}]]}`, want: "{deep_conflict:Array(Dynamic)[Dynamic<Array(Array(Nullable(Int64)))>,Dynamic<Array(Array(Nullable(String)))>] dyn_empty:Array(Array(Dynamic))[Array(Dynamic)[Dynamic<Int64>,Dynamic<String>],Array(Dynamic)[]] dyn_int:Array(Array(Dynamic))[Array(Dynamic)[Dynamic<Int64>,Dynamic<String>],Array(Dynamic)[Dynamic<Int64>]] dyn_str:Array(Array(Dynamic))[Array(Dynamic)[Dynamic<Int64>,Dynamic<String>],Array(Dynamic)[Dynamic<String>]] fb:Array(Array(Nullable(Float64))) ib:Array(Array(Nullable(Int64))) obj_deep:Array(Array(Array(JSON(max_dynamic_types=16, max_dynamic_paths=0))))[Array(Array(JSON(max_dynamic_types=16, max_dynamic_paths=0)))[Array(JSON(max_dynamic_types=16, max_dynamic_paths=0))[{k:Int64}]]] obj_int:Array(Dynamic)[Dynamic<Array(JSON)[{k:Int64}]>,Dynamic<Array(Nullable(Int64))>] obj_null:Array(Array(JSON(max_dynamic_types=16, max_dynamic_paths=0)))[Array(JSON(max_dynamic_types=16, max_dynamic_paths=0))[{k:Int64}],Array(JSON(max_dynamic_types=16, max_dynamic_paths=0))[]] obj_str_k:Array(Array(JSON(max_dynamic_types=16, max_dynamic_paths=0)))[Array(JSON(max_dynamic_types=16, max_dynamic_paths=0))[{k:Int64}],Array(JSON(max_dynamic_types=16, max_dynamic_paths=0))[{k:String}]] sb:Array(Array(Nullable(String)))}"},
		{name: "NestedArraysWithEmptyAndNull", doc: `{"en":[[1],[],null],"ne":[null,[1]],"nn":[[],null],"deep":[[[],[-1],[null,"123"]]],"mix":[null,["1e5"],[[],[true,1000000000000000],[false]],[-9223372036854775808]]}`, want: "{deep:Array(Array(Array(Dynamic)))[Array(Array(Dynamic))[Array(Dynamic)[],Array(Dynamic)[Dynamic<Int64>],Array(Dynamic)[null,Dynamic<String>]]] en:Array(Dynamic)[Dynamic<Array(Nullable(Int64))>,Dynamic<Array(Nullable(String))>,null] mix:Array(Array(Dynamic))[Array(Dynamic)[],Array(Dynamic)[Dynamic<String>],Array(Dynamic)[Dynamic<Array(Nullable(String))>,Dynamic<Array(Nullable(Int64))>,Dynamic<Array(Nullable(Bool))>],Array(Dynamic)[Dynamic<Int64>]] ne:Array(Array(Nullable(Int64))) nn:Array(Array(Nullable(String)))}"},
		{name: "ArrayDynamicNumbersUnify", doc: `{"fis":[-1e300,1,"123"],"sfi":["-1.5",0.1,9223372036854775807],"bis":[true,1,"x"],"eas":[[],[3],"s",[]]}`, want: "{bis:Array(Dynamic)[Dynamic<Bool>,Dynamic<Int64>,Dynamic<String>] eas:Array(Dynamic)[Dynamic<Array(Nullable(String))>,Dynamic<Array(Nullable(Int64))>,Dynamic<String>,Dynamic<Array(Nullable(String))>] fis:Array(Dynamic)[Dynamic<Float64>,Dynamic<Int64>,Dynamic<String>] sfi:Array(Dynamic)[Dynamic<String>,Dynamic<Float64>,Dynamic<Int64>]}"},
		{name: "NegativeAfterPositive_InNestedArrayWithUInt64", doc: `{"v":[[1,-1],[10000000000000000000]]}`, want: "{v:Array(Dynamic)[Dynamic<Array(Nullable(Int64))>,Dynamic<Array(Nullable(UInt64))>]}"},
		{name: "NegativeBeforePositive_InNestedArrayWithUInt64_TextFails", doc: `{"v":[[-1,1],[10000000000000000000]]}`, textFails: true, want: "{v:Array(Dynamic)[Dynamic<Array(Nullable(Int64))>,Dynamic<Array(Nullable(UInt64))>]}"},
		{name: "ArrayDynamicWithNestedJSONArrays", doc: `{"d":[1,{"a":[{"b":1}]},[{"c":2}]]}`, want: "{d:Array(Dynamic)[Dynamic<Int64>,Dynamic<{a:Array(JSON(max_dynamic_types=16, max_dynamic_paths=256))[{b:Int64}]}>,Dynamic<Array(JSON)[{c:Int64}]>]}"},
		{name: "ArrayBytes", fill: func(m pcommon.Map) { m.PutEmptySlice("b").AppendEmpty().SetEmptyBytes().FromRaw([]byte("hi")) }, want: "{b:Array(Nullable(String))}"},
		{name: "ArrayNaN_TextFails", fill: func(m pcommon.Map) {
			s := m.PutEmptySlice("f")
			s.AppendEmpty().SetDouble(math.NaN())
			s.AppendEmpty().SetDouble(1)
			s.AppendEmpty().SetDouble(2.5)
		}, textFails: true, want: "{f:Array(Nullable(Float64))}"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			m := pcommon.NewMap()
			if testCase.doc != "" {
				m = mapFromJSON(t, testCase.doc)
			}
			if testCase.fill != nil {
				testCase.fill(m)
			}
			obj := FromPcommonMap(m, Limits{MaxDynamicTypes: 32, MaxDynamicPaths: 0}, nil)
			assert.Equal(t, testCase.want, typeSignature(obj))
		})
	}
}

func TestFromPcommonMapNegativeIntegersNeverWidenedToUInt64(t *testing.T) {
	testCases := []struct {
		name string
		doc  string
		want []any
	}{
		{name: "NegativeFirst", doc: `{"v":[[-1,1],[10000000000000000000]]}`, want: []any{[]any{int64(-1), int64(1)}, []any{uint64(1e19)}}},
		{name: "NegativeLast", doc: `{"v":[[1,-1],[10000000000000000000]]}`, want: []any{[]any{int64(1), int64(-1)}, []any{uint64(1e19)}}},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			d := dynamicAt(t, FromPcommonMap(mapFromJSON(t, testCase.doc), Limits{MaxDynamicTypes: 32, MaxDynamicPaths: 0}, nil), "v")
			elems, ok := d.Any().([]any)
			require.True(t, ok)
			require.Len(t, elems, len(testCase.want))
			for i, el := range elems {
				assert.Equal(t, testCase.want[i], el.(chcol.Dynamic).Any())
			}
		})
	}
}

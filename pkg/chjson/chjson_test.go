package chjson

import (
	"math"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2/lib/chcol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func dynamicAt(t *testing.T, obj *chcol.JSON, path string) chcol.Dynamic {
	t.Helper()
	v, ok := obj.ValueAtPath(path)
	require.True(t, ok, "path %q missing", path)
	d, ok := v.(chcol.Dynamic)
	require.True(t, ok, "path %q is %T, want chcol.Dynamic", path, v)
	return d
}

func TestFromPcommonMapScalars(t *testing.T) {
	testCases := []struct {
		name      string
		fill      func(m pcommon.Map)
		wantType  string
		wantValue any
	}{
		{name: "String", fill: func(m pcommon.Map) { m.PutStr("v", "hello") }, wantType: "String", wantValue: "hello"},
		{name: "Int", fill: func(m pcommon.Map) { m.PutInt("v", -42) }, wantType: "Int64", wantValue: int64(-42)},
		{name: "Double", fill: func(m pcommon.Map) { m.PutDouble("v", 3.14) }, wantType: "Float64", wantValue: 3.14},
		{name: "Bool", fill: func(m pcommon.Map) { m.PutBool("v", true) }, wantType: "Bool", wantValue: true},
		{name: "Bytes_Base64String", fill: func(m pcommon.Map) { m.PutEmptyBytes("v").Append([]byte("hi")...) }, wantType: "String", wantValue: "aGk="},
		{name: "IntegralDouble_Int64", fill: func(m pcommon.Map) { m.PutDouble("v", 200) }, wantType: "Int64", wantValue: int64(200)},
		{name: "IntegralDoubleAboveInt64_UInt64", fill: func(m pcommon.Map) { m.PutDouble("v", 1e19) }, wantType: "UInt64", wantValue: uint64(1e19)},
		{name: "IntegralDoubleAboveUInt64_Float64", fill: func(m pcommon.Map) { m.PutDouble("v", 1.8446744073709552e19) }, wantType: "Float64", wantValue: 1.8446744073709552e19},
		{name: "DoubleFrom1e21_Float64", fill: func(m pcommon.Map) { m.PutDouble("v", 1e21) }, wantType: "Float64", wantValue: 1e21},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			m := pcommon.NewMap()
			testCase.fill(m)
			d := dynamicAt(t, FromPcommonMap(m, DefaultLimits, nil), "v")
			assert.Equal(t, testCase.wantType, d.Type())
			assert.Equal(t, testCase.wantValue, d.Any())
		})
	}
}

func TestFromPcommonMapNullLike(t *testing.T) {
	m := pcommon.NewMap()
	m.PutEmpty("null")
	m.PutDouble("nan", math.NaN())
	m.PutDouble("inf", math.Inf(1))

	paths := FromPcommonMap(m, DefaultLimits, nil).ValuesByPath()
	for _, path := range []string{"null", "nan", "inf"} {
		v, ok := paths[path]
		assert.True(t, ok, path)
		assert.Nil(t, v, path)
	}
}

func TestFromPcommonMapNestedMaps(t *testing.T) {
	m := pcommon.NewMap()
	nested := m.PutEmptyMap("a")
	nested.PutInt("b", 1)
	nested.PutEmptyMap("c").PutStr("d", "x")
	m.PutEmptyMap("empty")
	m.PutInt("a.b.dotted", 2)

	obj := FromPcommonMap(m, DefaultLimits, nil)

	assert.Equal(t, int64(1), dynamicAt(t, obj, "a.b").Any())
	assert.Equal(t, "x", dynamicAt(t, obj, "a.c.d").Any())
	assert.Equal(t, int64(2), dynamicAt(t, obj, "a.b.dotted").Any())
	_, hasEmpty := obj.ValueAtPath("empty")
	assert.False(t, hasEmpty, "empty maps produce no path")
	assert.Len(t, obj.ValuesByPath(), 3)
}

func TestFromPcommonMapDuplicatePaths(t *testing.T) {
	testCases := []struct {
		name      string
		fill      func(m pcommon.Map)
		wantType  string
		wantValue any
	}{
		{
			name:      "DottedKeyFirst_DottedWins",
			fill:      func(m pcommon.Map) { m.PutStr("db.function", "x"); m.PutEmptyMap("db").PutInt("function", 1) },
			wantType:  "String",
			wantValue: "x",
		},
		{
			name:      "NestedKeyFirst_NestedWins",
			fill:      func(m pcommon.Map) { m.PutEmptyMap("db").PutInt("function", 1); m.PutStr("db.function", "x") },
			wantType:  "Int64",
			wantValue: int64(1),
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			m := pcommon.NewMap()
			testCase.fill(m)
			obj := FromPcommonMap(m, DefaultLimits, nil)
			d := dynamicAt(t, obj, "db.function")
			assert.Equal(t, testCase.wantType, d.Type())
			assert.Equal(t, testCase.wantValue, d.Any())
			assert.Len(t, obj.ValuesByPath(), 1)
		})
	}
}

func TestFromPcommonMapTypedStringPaths(t *testing.T) {
	testCases := []struct {
		name string
		fill func(m pcommon.Map)
		want string
	}{
		{name: "Int_Stringified", fill: func(m pcommon.Map) { m.PutInt("message", 123) }, want: "123"},
		{name: "Map_SerializedAsJSON", fill: func(m pcommon.Map) { m.PutEmptyMap("message").PutInt("a", 1) }, want: `{"a":1}`},
		{name: "String_Unchanged", fill: func(m pcommon.Map) { m.PutStr("message", "plain") }, want: "plain"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			m := pcommon.NewMap()
			testCase.fill(m)
			obj := FromPcommonMap(m, DefaultLimits, map[string]struct{}{"message": {}})
			assert.Equal(t, testCase.want, obj.ValuesByPath()["message"])
			assert.Len(t, obj.ValuesByPath(), 1, "typed string path swallows the subtree")
		})
	}
}

func TestFromPcommonMapArrays(t *testing.T) {
	testCases := []struct {
		name      string
		elems     []any
		wantType  string
		wantElems []any
	}{
		{name: "Empty_String", elems: []any{}, wantType: "Array(Nullable(String))", wantElems: []any{}},
		{name: "OnlyNulls_String", elems: []any{nil}, wantType: "Array(Nullable(String))", wantElems: []any{nil}},
		{name: "Ints", elems: []any{int64(1), nil, int64(2)}, wantType: "Array(Nullable(Int64))", wantElems: []any{int64(1), nil, int64(2)}},
		{name: "Strings", elems: []any{"x"}, wantType: "Array(Nullable(String))", wantElems: []any{"x"}},
		{name: "Bools", elems: []any{true}, wantType: "Array(Nullable(Bool))", wantElems: []any{true}},
		{name: "IntFloat_Float64", elems: []any{int64(1), 2.5, nil}, wantType: "Array(Nullable(Float64))", wantElems: []any{float64(1), 2.5, nil}},
		{name: "IntBool_Int64", elems: []any{int64(7), true, false}, wantType: "Array(Nullable(Int64))", wantElems: []any{int64(7), int64(1), int64(0)}},
		{name: "FloatBool_Float64", elems: []any{0.5, true}, wantType: "Array(Nullable(Float64))", wantElems: []any{0.5, float64(1)}},
		{name: "IntegralDoubles_Int64", elems: []any{1.0, 2.0}, wantType: "Array(Nullable(Int64))", wantElems: []any{int64(1), int64(2)}},
		{name: "IntUInt64_UInt64", elems: []any{int64(1), 1e19}, wantType: "Array(Nullable(UInt64))", wantElems: []any{uint64(1), uint64(1e19)}},
		{name: "NegativeIntUInt64_Dynamic", elems: []any{int64(-1), 1e19}, wantType: "Array(Dynamic)", wantElems: []any{chcol.NewDynamicWithType(int64(-1), "Int64"), chcol.NewDynamicWithType(uint64(1e19), "UInt64")}},
		{name: "StringBool_String", elems: []any{true, "x"}, wantType: "Array(Nullable(String))", wantElems: []any{"true", "x"}},
		{name: "StringNumber_Dynamic", elems: []any{"x", int64(1)}, wantType: "Array(Dynamic)", wantElems: []any{chcol.NewDynamicWithType("x", "String"), chcol.NewDynamicWithType(int64(1), "Int64")}},
		{name: "NaN_Null", elems: []any{math.NaN(), 2.5}, wantType: "Array(Nullable(Float64))", wantElems: []any{nil, 2.5}},
		{name: "NestedIntsWithEmpty", elems: []any{[]any{int64(1)}, []any{}}, wantType: "Array(Array(Nullable(Int64)))", wantElems: []any{[]any{int64(1)}, []any{}}},
		{name: "NestedIntsWithNull", elems: []any{nil, []any{int64(1)}}, wantType: "Array(Array(Nullable(Int64)))", wantElems: []any{[]any{}, []any{int64(1)}}},
		{
			name:      "NestedIntsWithEmptyAndNull_Dynamic",
			elems:     []any{[]any{int64(1)}, []any{}, nil},
			wantType:  "Array(Dynamic)",
			wantElems: []any{chcol.NewDynamicWithType([]any{int64(1)}, "Array(Nullable(Int64))"), chcol.NewDynamicWithType([]any{}, "Array(Nullable(String))"), nil},
		},
		{name: "NestedIntFloat_Float64", elems: []any{[]any{int64(1)}, []any{2.5}}, wantType: "Array(Array(Nullable(Float64)))", wantElems: []any{[]any{float64(1)}, []any{2.5}}},
		{name: "NestedEmpty_String", elems: []any{[]any{}, []any{}}, wantType: "Array(Array(Nullable(String)))", wantElems: []any{[]any{}, []any{}}},
		{
			name:      "NestedConflict_OuterDynamic",
			elems:     []any{[]any{int64(1)}, []any{"a"}},
			wantType:  "Array(Dynamic)",
			wantElems: []any{chcol.NewDynamicWithType([]any{int64(1)}, "Array(Nullable(Int64))"), chcol.NewDynamicWithType([]any{"a"}, "Array(Nullable(String))")},
		},
		{
			name:     "InnerDynamic_Absorbs",
			elems:    []any{[]any{int64(1), "a"}, []any{int64(2)}},
			wantType: "Array(Array(Dynamic))",
			wantElems: []any{
				[]any{chcol.NewDynamicWithType(int64(1), "Int64"), chcol.NewDynamicWithType("a", "String")},
				[]any{chcol.NewDynamicWithType(int64(2), "Int64")},
			},
		},
		{name: "ScalarAndArray_Dynamic", elems: []any{int64(1), []any{int64(2)}}, wantType: "Array(Dynamic)", wantElems: []any{chcol.NewDynamicWithType(int64(1), "Int64"), chcol.NewDynamicWithType([]any{int64(2)}, "Array(Nullable(Int64))")}},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			m := pcommon.NewMap()
			require.NoError(t, m.PutEmptySlice("arr").FromRaw(testCase.elems))
			d := dynamicAt(t, FromPcommonMap(m, DefaultLimits, nil), "arr")
			assert.Equal(t, testCase.wantType, d.Type())
			assert.Equal(t, testCase.wantElems, d.Any())
		})
	}
}

func TestFromPcommonMapArrayOfObjects(t *testing.T) {
	testCases := []struct {
		name     string
		elems    []any
		wantType string
	}{
		{name: "Objects", elems: []any{map[string]any{"k": int64(1)}, map[string]any{"k": "x"}}, wantType: "Array(JSON(max_dynamic_types=16, max_dynamic_paths=256))"},
		{name: "ObjectsWithNull_Nullable", elems: []any{nil, map[string]any{"k": int64(1)}}, wantType: "Array(Nullable(JSON(max_dynamic_types=16, max_dynamic_paths=256)))"},
		{name: "ObjectsWithScalar_Dynamic", elems: []any{map[string]any{"k": int64(1)}, int64(2)}, wantType: "Array(Dynamic)"},
		{name: "NestedArrayOfObjects_SameLimits", elems: []any{[]any{map[string]any{"k": int64(1)}}}, wantType: "Array(Array(JSON(max_dynamic_types=16, max_dynamic_paths=256)))"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			m := pcommon.NewMap()
			require.NoError(t, m.PutEmptySlice("objs").FromRaw(testCase.elems))
			assert.Equal(t, testCase.wantType, dynamicAt(t, FromPcommonMap(m, DefaultLimits, nil), "objs").Type())
		})
	}
}

func TestFromPcommonMapArrayElementsKeepOwnTypes(t *testing.T) {
	m := pcommon.NewMap()
	require.NoError(t, m.PutEmptySlice("objs").FromRaw([]any{
		map[string]any{"k": int64(1)},
		map[string]any{"k": 2.5},
		nil,
	}))

	elems := dynamicAt(t, FromPcommonMap(m, DefaultLimits, nil), "objs").Any().([]any)
	require.Len(t, elems, 3)
	assert.Equal(t, chcol.NewDynamicWithType(int64(1), "Int64"), elems[0].(*chcol.JSON).ValuesByPath()["k"])
	assert.Equal(t, chcol.NewDynamicWithType(2.5, "Float64"), elems[1].(*chcol.JSON).ValuesByPath()["k"])
	assert.Equal(t, (*chcol.JSON)(nil), elems[2])
}

func TestFromPcommonMapDynamicElements(t *testing.T) {
	m := pcommon.NewMap()
	require.NoError(t, m.PutEmptySlice("mixed").FromRaw([]any{
		int64(1),
		"a",
		nil,
		2.5,
		true,
		map[string]any{"x": int64(2)},
		[]any{int64(3)},
		[]any{map[string]any{"deep": []any{map[string]any{"y": int64(4)}}}},
	}))

	d := dynamicAt(t, FromPcommonMap(m, DefaultLimits, nil), "mixed")
	require.Equal(t, "Array(Dynamic)", d.Type())
	elems := d.Any().([]any)
	wantTypes := []string{"Int64", "String", "", "Float64", "Bool", "JSON", "Array(Nullable(Int64))", "Array(JSON)"}
	require.Len(t, elems, len(wantTypes))
	for i, want := range wantTypes {
		if want == "" {
			assert.Nil(t, elems[i])
			continue
		}
		el, ok := elems[i].(chcol.Dynamic)
		require.True(t, ok, "element %d is %T", i, elems[i])
		assert.Equal(t, want, el.Type(), "element %d", i)
	}
	nestedObj := elems[7].(chcol.Dynamic).Any().([]any)[0].(*chcol.JSON)
	assert.Equal(t, "Array(JSON(max_dynamic_types=16, max_dynamic_paths=256))", nestedObj.ValuesByPath()["deep"].(chcol.Dynamic).Type(), "objects inside Dynamic use default limits")
}

func TestFromPcommonMapArrayJSONLimits(t *testing.T) {
	m := pcommon.NewMap()
	level1 := m.PutEmptySlice("a").AppendEmpty().SetEmptyMap()
	level2 := level1.PutEmptySlice("b").AppendEmpty().SetEmptyMap()
	level2.PutEmptySlice("c").AppendEmpty().SetEmptyMap().PutInt("k", 1)

	testCases := []struct {
		name   string
		limits Limits
		want   []string
	}{
		{
			name:   "DefaultColumn",
			limits: DefaultLimits,
			want: []string{
				"Array(JSON(max_dynamic_types=16, max_dynamic_paths=256))",
				"Array(JSON(max_dynamic_types=8, max_dynamic_paths=64))",
				"Array(JSON(max_dynamic_types=4, max_dynamic_paths=16))",
			},
		},
		{
			name:   "ZeroDynamicPathsColumn",
			limits: Limits{MaxDynamicTypes: 32, MaxDynamicPaths: 0},
			want: []string{
				"Array(JSON(max_dynamic_types=16, max_dynamic_paths=0))",
				"Array(JSON(max_dynamic_types=8, max_dynamic_paths=0))",
				"Array(JSON(max_dynamic_types=4, max_dynamic_paths=0))",
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			a := dynamicAt(t, FromPcommonMap(m, testCase.limits, nil), "a")
			assert.Equal(t, testCase.want[0], a.Type())
			b := a.Any().([]any)[0].(*chcol.JSON).ValuesByPath()["b"].(chcol.Dynamic)
			assert.Equal(t, testCase.want[1], b.Type())
			c := b.Any().([]any)[0].(*chcol.JSON).ValuesByPath()["c"].(chcol.Dynamic)
			assert.Equal(t, testCase.want[2], c.Type())
		})
	}
}

func TestPathBudget(t *testing.T) {
	obj1 := chcol.NewJSON()
	obj1.SetValueAtPath("a", int64(1))
	obj1.SetValueAtPath("b", int64(2))
	obj2 := chcol.NewJSON()
	obj2.SetValueAtPath("b", int64(3))
	obj2.SetValueAtPath("c", int64(4))

	seen := map[string]struct{}{}
	assert.False(t, ExceedsPathBudget(obj1, seen, 1), "empty batch never exceeds, even over budget")
	RecordPaths(obj1, seen)
	assert.Len(t, seen, 2)

	assert.False(t, ExceedsPathBudget(obj2, seen, 3), "one new path fits budget 3")
	assert.True(t, ExceedsPathBudget(obj2, seen, 2), "one new path exceeds budget 2")
	RecordPaths(obj2, seen)
	assert.Len(t, seen, 3)
}

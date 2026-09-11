package chjson

import (
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2/lib/chcol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestFromPcommonMapScalars(t *testing.T) {
	m := pcommon.NewMap()
	m.PutStr("str", "hello")
	m.PutInt("int", 42)
	m.PutDouble("double", 3.14)
	m.PutDouble("bigdouble", 1.8446744073709552e19)
	m.PutBool("bool", true)
	m.PutEmpty("null")
	m.PutEmptyBytes("bytes").Append([]byte("hi")...)

	obj := FromPcommonMap(m, nil)
	paths := obj.ValuesByPath()

	assert.Equal(t, "hello", paths["str"])
	assert.Equal(t, int64(42), paths["int"])
	assert.Equal(t, 3.14, paths["double"])
	assert.Equal(t, 1.8446744073709552e19, paths["bigdouble"])
	assert.Equal(t, true, paths["bool"])
	assert.Nil(t, paths["null"])
	assert.Equal(t, "aGk=", paths["bytes"])
}

func TestFromPcommonMapNestedMaps(t *testing.T) {
	m := pcommon.NewMap()
	nested := m.PutEmptyMap("a")
	nested.PutInt("b", 1)
	deep := nested.PutEmptyMap("c")
	deep.PutStr("d", "x")
	m.PutEmptyMap("empty")
	m.PutInt("a.b.dotted", 2)

	obj := FromPcommonMap(m, nil)
	paths := obj.ValuesByPath()

	assert.Equal(t, int64(1), paths["a.b"])
	assert.Equal(t, "x", paths["a.c.d"])
	assert.Equal(t, int64(2), paths["a.b.dotted"])
	_, hasEmpty := obj.ValueAtPath("empty")
	assert.False(t, hasEmpty, "empty maps produce no path, matching server-side inference")
	assert.Len(t, paths, 3)
}

func TestFromPcommonMapTypedStringPaths(t *testing.T) {
	m := pcommon.NewMap()
	m.PutInt("message", 123)
	obj := FromPcommonMap(m, map[string]struct{}{"message": {}})
	assert.Equal(t, "123", obj.ValuesByPath()["message"])

	m2 := pcommon.NewMap()
	inner := m2.PutEmptyMap("message")
	inner.PutInt("a", 1)
	obj2 := FromPcommonMap(m2, map[string]struct{}{"message": {}})
	assert.Equal(t, `{"a":1}`, obj2.ValuesByPath()["message"])
	_, hasNested := obj2.ValueAtPath("message.a")
	assert.False(t, hasNested, "typed string path swallows the subtree, matching server coercion")

	m3 := pcommon.NewMap()
	m3.PutStr("message", "plain")
	obj3 := FromPcommonMap(m3, map[string]struct{}{"message": {}})
	assert.Equal(t, "plain", obj3.ValuesByPath()["message"])
}

func dynamicAt(t *testing.T, obj *chcol.JSON, path string) chcol.Dynamic {
	t.Helper()
	v, ok := obj.ValueAtPath(path)
	require.True(t, ok, "path %q missing", path)
	d, ok := v.(chcol.Dynamic)
	require.True(t, ok, "path %q is %T, want chcol.Dynamic", path, v)
	return d
}

func TestFromPcommonMapScalarArrays(t *testing.T) {
	m := pcommon.NewMap()
	ints := m.PutEmptySlice("ints")
	ints.AppendEmpty().SetInt(1)
	ints.AppendEmpty()
	ints.AppendEmpty().SetInt(2)
	strs := m.PutEmptySlice("strs")
	strs.AppendEmpty().SetStr("x")
	floats := m.PutEmptySlice("floats")
	floats.AppendEmpty().SetDouble(1.5)
	bools := m.PutEmptySlice("bools")
	bools.AppendEmpty().SetBool(true)
	m.PutEmptySlice("empty")
	nulls := m.PutEmptySlice("nulls")
	nulls.AppendEmpty()

	obj := FromPcommonMap(m, nil)

	d := dynamicAt(t, obj, "ints")
	assert.Equal(t, "Array(Nullable(Int64))", d.Type())
	assert.Equal(t, []any{int64(1), nil, int64(2)}, d.Any())

	assert.Equal(t, "Array(Nullable(String))", dynamicAt(t, obj, "strs").Type())
	assert.Equal(t, "Array(Nullable(Float64))", dynamicAt(t, obj, "floats").Type())
	assert.Equal(t, "Array(Nullable(Bool))", dynamicAt(t, obj, "bools").Type())

	e := dynamicAt(t, obj, "empty")
	assert.Equal(t, "Array(Nullable(String))", e.Type())
	assert.Empty(t, e.Any())

	n := dynamicAt(t, obj, "nulls")
	assert.Equal(t, "Array(Nullable(String))", n.Type())
	assert.Equal(t, []any{nil}, n.Any())
}

func TestFromPcommonMapArrayOfObjects(t *testing.T) {
	m := pcommon.NewMap()
	objs := m.PutEmptySlice("objs")
	first := objs.AppendEmpty().SetEmptyMap()
	first.PutInt("k", 1)
	innerArr := first.PutEmptySlice("inner")
	innerArr.AppendEmpty().SetInt(9)
	second := objs.AppendEmpty().SetEmptyMap()
	second.PutInt("k", 2)

	obj := FromPcommonMap(m, nil)
	d := dynamicAt(t, obj, "objs")
	assert.Equal(t, "Array(JSON)", d.Type())

	elems, ok := d.Any().([]any)
	require.True(t, ok)
	require.Len(t, elems, 2)
	el1, ok := elems[0].(*chcol.JSON)
	require.True(t, ok)
	assert.Equal(t, int64(1), el1.ValuesByPath()["k"])
	inner := el1.ValuesByPath()["inner"].(chcol.Dynamic)
	assert.Equal(t, "Array(Nullable(Int64))", inner.Type())
}

func TestFromPcommonMapMixedArrays(t *testing.T) {
	m := pcommon.NewMap()
	mixed := m.PutEmptySlice("mixed")
	mixed.AppendEmpty().SetInt(1)
	mixed.AppendEmpty().SetStr("a")
	mixed.AppendEmpty()
	mixed.AppendEmpty().SetEmptyMap().PutInt("x", 2)
	nestedArr := mixed.AppendEmpty().SetEmptySlice()
	nestedArr.AppendEmpty().SetInt(3)

	objsWithNull := m.PutEmptySlice("objs_with_null")
	objsWithNull.AppendEmpty()
	objsWithNull.AppendEmpty().SetEmptyMap().PutInt("x", 1)

	intFloat := m.PutEmptySlice("int_float")
	intFloat.AppendEmpty().SetInt(1)
	intFloat.AppendEmpty().SetDouble(2.5)

	obj := FromPcommonMap(m, nil)

	d := dynamicAt(t, obj, "mixed")
	require.Equal(t, "Array(Dynamic)", d.Type())
	elems := d.Any().([]any)
	require.Len(t, elems, 5)
	assert.Equal(t, int64(1), elems[0])
	assert.Equal(t, "a", elems[1])
	assert.Nil(t, elems[2])
	mapElem, ok := elems[3].(chcol.Dynamic)
	require.True(t, ok)
	assert.Equal(t, "JSON", mapElem.Type())
	assert.Equal(t, int64(2), mapElem.Any().(*chcol.JSON).ValuesByPath()["x"])
	arrElem, ok := elems[4].(chcol.Dynamic)
	require.True(t, ok)
	assert.Equal(t, "Array(Nullable(Int64))", arrElem.Type())

	assert.Equal(t, "Array(Dynamic)", dynamicAt(t, obj, "objs_with_null").Type())
	assert.Equal(t, "Array(Dynamic)", dynamicAt(t, obj, "int_float").Type())
	ifElems := dynamicAt(t, obj, "int_float").Any().([]any)
	assert.Equal(t, int64(1), ifElems[0])
	assert.Equal(t, 2.5, ifElems[1])
}

package jsontypeexporter

import (
	"reflect"
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

var kindToTypes = map[reflect.Kind]string{
	reflect.String:  StringType,
	reflect.Int64:   IntType,
	reflect.Float64: Float64Type,
	reflect.Bool:    BooleanType,
}

var kindToArrayTypes = map[reflect.Kind]string{
	reflect.String:  ArrayString,
	reflect.Int64:   ArrayInt,
	reflect.Int:     ArrayInt,
	reflect.Int8:    ArrayInt,
	reflect.Int16:   ArrayInt,
	reflect.Int32:   ArrayInt,
	reflect.Float64: ArrayFloat64,
	reflect.Float32: ArrayFloat64,
	reflect.Bool:    ArrayBoolean,
	reflect.Map:     ArrayJSON,
}

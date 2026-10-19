package chjson

import (
	"fmt"
	"math"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// Port of ClickHouse 25.12 per-value JSON type inference (JSONExtractTree.cpp DynamicNode::elementToDataType,
// SchemaInferenceUtils.cpp, transformTypesRecursively.cpp) under the server's default settings.

type typeID uint8

const (
	tNothing typeID = iota
	tBool
	tInt64
	tUInt64
	tFloat64
	tString
	tObject
	tDynamic
	tNullable
	tArray
	tTuple
)

type chType struct {
	id       typeID
	nested   *chType
	elems    []*chType
	negative bool
}

var (
	nothingType = &chType{id: tNothing}
	boolType    = &chType{id: tBool}
	uint64Type  = &chType{id: tUInt64}
	float64Type = &chType{id: tFloat64}
	stringType  = &chType{id: tString}
	objectType  = &chType{id: tObject}
	dynamicType = &chType{id: tDynamic}
)

func int64Type(negative bool) *chType      { return &chType{id: tInt64, negative: negative} }
func arrayOf(nested *chType) *chType       { return &chType{id: tArray, nested: nested} }
func tupleOf(elems []*chType) *chType      { return &chType{id: tTuple, elems: elems} }
func isNothing(t *chType) bool             { return t.id == tNothing }
func canBeInsideNullable(t *chType) bool   { return t.id <= tObject }
func cloneTypes(types []*chType) []*chType { return append([]*chType(nil), types...) }
func typeIndexes(types []*chType) idxSet   { return newIdxSet(types) }
func makeNullable(t *chType) *chType {
	if t.id == tNullable {
		return t
	}
	return &chType{id: tNullable, nested: t}
}

func removeNullable(t *chType) *chType {
	if t.id == tNullable {
		return t.nested
	}
	return t
}

type idxSet map[typeID]bool

func newIdxSet(types []*chType) idxSet {
	s := idxSet{}
	for _, t := range types {
		s[t.id] = true
	}
	return s
}

func equalTypes(a, b *chType) bool {
	if a.id != b.id {
		return false
	}
	switch a.id {
	case tNullable, tArray:
		return equalTypes(a.nested, b.nested)
	case tTuple:
		if len(a.elems) != len(b.elems) {
			return false
		}
		for i := range a.elems {
			if !equalTypes(a.elems[i], b.elems[i]) {
				return false
			}
		}
	}
	return true
}

func allEqual(types []*chType) bool {
	for _, t := range types[1:] {
		if !equalTypes(types[0], t) {
			return false
		}
	}
	return true
}

func isComplete(t *chType) bool {
	switch t.id {
	case tNothing:
		return false
	case tNullable, tArray:
		return isComplete(t.nested)
	case tTuple:
		for _, e := range t.elems {
			if !isComplete(e) {
				return false
			}
		}
	}
	return true
}

// collapse returns the common type of structurally equal types. An integer stays negative if any
// collapsed value was negative, so a negative value is never widened to UInt64.
func collapse(types []*chType) *chType {
	last := types[len(types)-1]
	switch last.id {
	case tInt64:
		for _, t := range types {
			if t.negative {
				return int64Type(true)
			}
		}
	case tNullable, tArray:
		nested := make([]*chType, len(types))
		for i, t := range types {
			nested[i] = t.nested
		}
		return &chType{id: last.id, nested: collapse(nested)}
	case tTuple:
		elems := make([]*chType, len(last.elems))
		for e := range last.elems {
			column := make([]*chType, len(types))
			for i, t := range types {
				column[i] = t.elems[e]
			}
			elems[e] = collapse(column)
		}
		return tupleOf(elems)
	}
	return last
}

// inferValueType infers from the JSON text encoding/json produces for v.
func inferValueType(v pcommon.Value) *chType {
	switch v.Type() {
	case pcommon.ValueTypeStr, pcommon.ValueTypeBytes:
		return stringType
	case pcommon.ValueTypeBool:
		return boolType
	case pcommon.ValueTypeInt:
		return int64Type(v.Int() < 0)
	case pcommon.ValueTypeDouble:
		return doubleLiteralType(v.Double())
	case pcommon.ValueTypeMap:
		return objectType
	case pcommon.ValueTypeSlice:
		return inferArrayType(v.Slice())
	default:
		return makeNullable(nothingType)
	}
}

func inferArrayType(s pcommon.Slice) *chType {
	types := make([]*chType, 0, s.Len())
	for _, el := range s.All() {
		types = append(types, inferValueType(el))
	}
	if len(types) == 0 {
		return arrayOf(nothingType)
	}
	if allEqual(types) {
		return arrayOf(collapse(types))
	}
	for _, t := range types {
		if !isComplete(t) {
			return tupleOf(types)
		}
	}
	transformed := cloneTypes(types)
	transformTypesRecursively(transformed)
	if allEqual(transformed) {
		return arrayOf(collapse(transformed))
	}
	return tupleOf(types)
}

// doubleLiteralType follows the JSON text: encoding/json writes integral doubles below 1e21 without a
// fraction, which ClickHouse reads as Int64 or UInt64. NaN/Inf have no JSON form and are stored as null.
func doubleLiteralType(f float64) *chType {
	switch {
	case math.IsNaN(f) || math.IsInf(f, 0):
		return makeNullable(nothingType)
	case math.Trunc(f) != f || math.Abs(f) >= 1e21:
		return float64Type
	case f >= math.MinInt64 && f < math.MaxInt64:
		return int64Type(f < 0)
	case f > 0 && f < math.MaxUint64:
		return uint64Type
	default:
		return float64Type
	}
}

func elementType(v pcommon.Value) *chType {
	t := transformFinalType(inferValueType(v), false)
	if t.id == tNullable || t.id == tArray || t.id == tTuple {
		t = makeNullableRecursively(t)
	}
	return removeNullable(t)
}

func makeNullableRecursively(t *chType) *chType {
	switch t.id {
	case tNullable, tObject, tDynamic:
		return t
	case tArray:
		return arrayOf(makeNullableRecursively(t.nested))
	case tTuple:
		elems := make([]*chType, len(t.elems))
		for i, e := range t.elems {
			elems[i] = makeNullableRecursively(e)
		}
		return tupleOf(elems)
	default:
		return makeNullable(t)
	}
}

func transformTypesRecursively(types []*chType) {
	idx := typeIndexes(types)
	switch {
	case idx[tNullable]:
		wasNullable := make([]bool, len(types))
		nested := make([]*chType, len(types))
		for i, t := range types {
			wasNullable[i] = t.id == tNullable
			nested[i] = removeNullable(t)
		}
		transformTypesRecursively(nested)
		for i := range types {
			if wasNullable[i] && canBeInsideNullable(nested[i]) {
				types[i] = makeNullable(nested[i])
			} else {
				types[i] = nested[i]
			}
		}
		transformComplexTypes(types, typeIndexes(types))
	case idx[tArray]:
		if len(idx) == 1 {
			nested := make([]*chType, len(types))
			for i, t := range types {
				nested[i] = t.nested
			}
			transformTypesRecursively(nested)
			for i := range types {
				types[i] = arrayOf(nested[i])
			}
		}
		transformComplexTypes(types, idx)
	case idx[tTuple]:
		if len(idx) == 1 && tupleSizesEqual(types) {
			size := len(types[0].elems)
			rebuilt := make([][]*chType, len(types))
			for e := 0; e < size; e++ {
				column := make([]*chType, len(types))
				for i, t := range types {
					column[i] = t.elems[e]
				}
				transformTypesRecursively(column)
				for i := range types {
					rebuilt[i] = append(rebuilt[i], column[i])
				}
			}
			for i := range types {
				types[i] = tupleOf(rebuilt[i])
			}
		}
		transformComplexTypes(types, idx)
	default:
		transformSimpleTypes(types)
	}
}

func tupleSizesEqual(types []*chType) bool {
	for _, t := range types {
		if len(t.elems) != len(types[0].elems) {
			return false
		}
	}
	return true
}

func transformSimpleTypes(types []*chType) {
	transformNothingSimpleTypes(types)
	transformIntegers(types)
	transformIntegersAndFloatsToFloats(types)
	transformBoolsAndNumbersToNumbers(types)
	transformBoolsAndStringsToString(types)
}

// transformNothingSimpleTypes replaces Nothing with the first other type.
func transformNothingSimpleTypes(types []*chType) {
	idx := typeIndexes(types)
	if !idx[tNothing] || len(idx) == 1 {
		return
	}
	var first *chType
	for _, t := range types {
		if !isNothing(t) {
			first = t
			break
		}
	}
	for i, t := range types {
		if isNothing(t) {
			types[i] = first
		}
	}
}

func transformIntegers(types []*chType) {
	if idx := typeIndexes(types); !idx[tInt64] || !idx[tUInt64] {
		return
	}
	for i, t := range types {
		if t.id == tInt64 && !t.negative {
			types[i] = uint64Type
		}
	}
}

func transformIntegersAndFloatsToFloats(types []*chType) {
	if idx := typeIndexes(types); !idx[tFloat64] || (!idx[tInt64] && !idx[tUInt64]) {
		return
	}
	for i, t := range types {
		if t.id == tInt64 || t.id == tUInt64 {
			types[i] = float64Type
		}
	}
}

func transformBoolsAndNumbersToNumbers(types []*chType) {
	idx := typeIndexes(types)
	if !idx[tBool] || (!idx[tInt64] && !idx[tUInt64] && !idx[tFloat64]) {
		return
	}
	to := float64Type
	switch {
	case idx[tInt64]:
		to = int64Type(false)
	case idx[tUInt64]:
		to = uint64Type
	}
	for i, t := range types {
		if t.id == tBool {
			types[i] = to
		}
	}
}

func transformBoolsAndStringsToString(types []*chType) {
	if idx := typeIndexes(types); !idx[tBool] || !idx[tString] {
		return
	}
	for i, t := range types {
		if t.id == tBool {
			types[i] = stringType
		}
	}
}

// transformComplexTypes keeps idx the way ClickHouse maintains type_indexes: tuples turned into arrays
// do not add Array to it.
func transformComplexTypes(types []*chType, idx idxSet) {
	if transformNullableTypes(types, idx) {
		idx = typeIndexes(types)
	}
	if transformNothingComplexTypes(types) {
		idx = typeIndexes(types)
	}
	if !idx[tTuple] || transformTuplesWithEqualElementsToArrays(types) {
		return
	}
	if transformTuplesAndArraysToArrays(types, idx) {
		return
	}
	transformTuplesAndArraysToArrayOfDynamic(types)
}

func transformNullableTypes(types []*chType, idx idxSet) bool {
	if !idx[tNullable] {
		return false
	}
	for i, t := range types {
		if canBeInsideNullable(t) {
			types[i] = makeNullable(t)
		}
	}
	return true
}

// transformNothingComplexTypes replaces Nothing and Nullable(Nothing) with the last other type.
func transformNothingComplexTypes(types []*chType) bool {
	var last *chType
	haveNothing := false
	for _, t := range types {
		if isNothing(removeNullable(t)) {
			haveNothing = true
		} else {
			last = t
		}
	}
	if !haveNothing || last == nil {
		return false
	}
	for i, t := range types {
		if isNothing(removeNullable(t)) {
			types[i] = last
		}
	}
	return true
}

func transformTuplesWithEqualElementsToArrays(types []*chType) (allConverted bool) {
	allConverted = true
	for i, t := range types {
		if t.id != tTuple {
			continue
		}
		if allEqual(t.elems) {
			types[i] = arrayOf(collapse(t.elems))
		} else {
			allConverted = false
		}
	}
	return allConverted
}

func transformTuplesAndArraysToArrays(types []*chType, idx idxSet) bool {
	sizesEqual, size := true, -1
	for _, t := range types {
		if t.id == tTuple {
			if size == -1 {
				size = len(t.elems)
			} else if len(t.elems) != size {
				sizesEqual = false
			}
		}
	}
	if !idx[tArray] && !sizesEqual {
		return false
	}
	var nested []*chType
	for _, t := range types {
		switch t.id {
		case tArray:
			nested = append(nested, t.nested)
		case tTuple:
			nested = append(nested, t.elems...)
		}
	}
	transformTypesRecursively(nested)
	if !allEqual(nested) {
		return false
	}
	common := collapse(nested)
	for i, t := range types {
		if t.id == tArray || t.id == tTuple {
			types[i] = arrayOf(common)
		}
	}
	return true
}

func transformTuplesAndArraysToArrayOfDynamic(types []*chType) {
	for i, t := range types {
		if t.id == tArray || t.id == tTuple {
			types[i] = arrayOf(dynamicType)
		}
	}
}

// transformFinalType resolves leftover tuples to arrays and Nothing to String; keepNothing defers the
// latter while it is still unknown whether a tuple becomes an array.
func transformFinalType(t *chType, keepNothing bool) *chType {
	switch t.id {
	case tNothing:
		if keepNothing {
			return t
		}
		return stringType
	case tNullable:
		return makeNullable(transformFinalType(t.nested, keepNothing))
	case tArray:
		return arrayOf(transformFinalType(t.nested, keepNothing))
	case tTuple:
		transformed := cloneTypes(t.elems)
		transformTypesRecursively(transformed)
		if allEqual(transformed) {
			return transformFinalType(arrayOf(collapse(transformed)), false)
		}
		finalized := make([]*chType, len(t.elems))
		for i, e := range t.elems {
			finalized[i] = transformFinalType(e, true)
		}
		transformTypesRecursively(finalized)
		if allEqual(finalized) {
			return arrayOf(collapse(finalized))
		}
		return arrayOf(dynamicType)
	default:
		return t
	}
}

// typeName renders t; objects get objectLimits, or plain JSON when nil.
func typeName(t *chType, objectLimits *Limits) string {
	switch t.id {
	case tBool:
		return "Bool"
	case tInt64:
		return "Int64"
	case tUInt64:
		return "UInt64"
	case tFloat64:
		return "Float64"
	case tObject:
		if objectLimits == nil {
			return "JSON"
		}
		return fmt.Sprintf("JSON(max_dynamic_types=%d, max_dynamic_paths=%d)", objectLimits.MaxDynamicTypes, objectLimits.MaxDynamicPaths)
	case tDynamic:
		return "Dynamic"
	case tNullable:
		return "Nullable(" + typeName(t.nested, objectLimits) + ")"
	case tArray:
		return "Array(" + typeName(t.nested, objectLimits) + ")"
	case tTuple:
		names := make([]string, len(t.elems))
		for i, e := range t.elems {
			names[i] = typeName(e, objectLimits)
		}
		return "Tuple(" + strings.Join(names, ", ") + ")"
	default:
		return "String"
	}
}

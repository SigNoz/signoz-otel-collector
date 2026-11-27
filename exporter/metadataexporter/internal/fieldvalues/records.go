package fieldvalues

import (
	"strconv"
	"sync"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// The input of a push is built from the pdata before the writer lock is
// taken: the pairs of each record with their hashes. Under the lock, only the
// value rules and the key cache are left.

type pairRange struct {
	lo, hi int
}

// preparedRecord is a log record or span: its record pairs are
// pairs[lo:mid], its event pairs pairs[mid:hi].
type preparedRecord struct {
	resource     int
	ts, fallback pcommon.Timestamp
	lo, mid, hi  int
}

type recordsInput struct {
	pairs     []pair
	resources []pairRange
	records   []preparedRecord
}

// maxPooledPairs and maxPooledRows keep a large input or batch out of the
// pools, so that a large push does not hold its memory after it is done:
// 65,536 pairs are 4.5 MiB, and 16,384 rows 1.9 MiB.
const (
	maxPooledPairs = 1 << 16
	maxPooledRows  = 1 << 14
)

var recordsPool = sync.Pool{New: func() any { return &recordsInput{} }}

func getRecordsInput() *recordsInput {
	return recordsPool.Get().(*recordsInput)
}

func putRecordsInput(in *recordsInput) {
	if cap(in.pairs) > maxPooledPairs {
		return
	}
	clear(in.pairs)
	in.pairs = in.pairs[:0]
	in.resources = in.resources[:0]
	in.records = in.records[:0]
	recordsPool.Put(in)
}

func (b *batch) addRecords(in *recordsInput) {
	for range in.resources {
		b.refs = append(b.refs, resourceRef{})
	}
	for i := range in.records {
		r := &in.records[i]
		seen := b.seenMillis(r.ts, r.fallback)
		ref := &b.refs[r.resource]
		if !ref.ready {
			res := in.resources[r.resource]
			*ref = b.resource(in.pairs[res.lo:res.hi], seen)
		}
		b.record(*ref, in.pairs[r.lo:r.mid], in.pairs[r.mid:r.hi], seen)
	}
}

// appendAttrPairs converts attributes to pairs, with the names and the types
// under which the exporter of the signal stores them. A reader asks for a
// field by the name and the data type of its key, so they must be the same
// here. A top-level key equal to skip is left out.
//
//   - Resource, scope and event attributes are strings, whatever their type.
//   - The attributes of a log record or a span keep numbers and bools. All
//     other types are strings.
//   - flattenMaps is for spans: the traces exporter stores a map attribute as
//     one attribute for each leaf, with a dotted name. The logs exporter
//     stores a map as one JSON string.
func appendAttrPairs(dst []pair, ctx fieldContext, m pcommon.Map, skip string, flattenMaps bool) []pair {
	m.Range(func(k string, v pcommon.Value) bool {
		switch {
		case skip != "" && k == skip:
		case ctx != contextAttribute:
			dst = append(dst, stringPair(ctx, k, v.AsString()))
		case flattenMaps && v.Type() == pcommon.ValueTypeMap:
			dst = appendFlatPairs(dst, ctx, k, v)
		default:
			dst = appendAttrPair(dst, ctx, k, v)
		}
		return true
	})
	return dst
}

func appendAttrPair(dst []pair, ctx fieldContext, name string, v pcommon.Value) []pair {
	switch v.Type() {
	case pcommon.ValueTypeInt:
		return append(dst, intPair(ctx, name, v.Int()))
	case pcommon.ValueTypeDouble:
		return append(dst, numberPair(ctx, name, v.Double()))
	case pcommon.ValueTypeBool:
		return append(dst, boolPair(ctx, name, v.Bool()))
	}
	return append(dst, stringPair(ctx, name, v.AsString()))
}

// appendFlatPairs gives the leaves of a map attribute of a span, as
// utils/flatten names them: a dot between the levels, and the index for the
// items of a slice.
func appendFlatPairs(dst []pair, ctx fieldContext, name string, v pcommon.Value) []pair {
	switch v.Type() {
	case pcommon.ValueTypeMap:
		v.Map().Range(func(k string, child pcommon.Value) bool {
			dst = appendFlatPairs(dst, ctx, name+"."+k, child)
			return true
		})
		return dst
	case pcommon.ValueTypeSlice:
		items := v.Slice()
		for i := 0; i < items.Len(); i++ {
			dst = appendFlatPairs(dst, ctx, name+"."+strconv.Itoa(i), items.At(i))
		}
		return dst
	}
	return appendAttrPair(dst, ctx, name, v)
}

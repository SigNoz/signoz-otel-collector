package fieldvalues

import (
	"math"
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

// maxPooledPairs keeps a very large input out of the pool, so that one large
// push does not hold its memory after it is done.
const maxPooledPairs = 1 << 20

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

// appendAttrPairs converts attributes to pairs. Nested maps become dotted
// names; slices are kept as their JSON string. A top-level key equal to skip
// is left out.
func appendAttrPairs(dst []pair, ctx fieldContext, m pcommon.Map, skip string) []pair {
	m.Range(func(k string, v pcommon.Value) bool {
		if skip == "" || k != skip {
			dst = appendAttrPair(dst, ctx, k, v)
		}
		return true
	})
	return dst
}

func appendAttrPair(dst []pair, ctx fieldContext, name string, v pcommon.Value) []pair {
	switch v.Type() {
	case pcommon.ValueTypeStr:
		dst = append(dst, stringPair(ctx, name, v.Str()))
	case pcommon.ValueTypeInt:
		dst = append(dst, numberPair(ctx, name, float64(v.Int())))
	case pcommon.ValueTypeDouble:
		if f := v.Double(); !math.IsNaN(f) && !math.IsInf(f, 0) {
			dst = append(dst, numberPair(ctx, name, f))
		}
	case pcommon.ValueTypeBool:
		dst = append(dst, boolPair(ctx, name, v.Bool()))
	case pcommon.ValueTypeMap:
		v.Map().Range(func(k string, child pcommon.Value) bool {
			dst = appendAttrPair(dst, ctx, name+"."+k, child)
			return true
		})
	case pcommon.ValueTypeSlice:
		dst = append(dst, stringPair(ctx, name, v.AsString()))
	}
	return dst
}

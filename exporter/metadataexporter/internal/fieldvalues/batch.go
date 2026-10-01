package fieldvalues

import (
	"math"
	"strconv"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

type leftOutReason string

const (
	reasonValueLength  leftOutReason = "value_length"
	reasonFieldPlaces  leftOutReason = "field_places"
	reasonSampleBudget leftOutReason = "sample_budget"
	reasonCacheFull    leftOutReason = "cache_full"
)

// mode is what a pair of a record does.
type mode uint8

const (
	modeSkip    mode = iota
	modeInHash       // part of the set hash, or of the resource identity
	modeOutside      // outside the hash, written once per set (or resource) and value per day
	modeSample       // outside the hash, written once per value per day
)

const (
	dayMillis           = uint64(24 * 60 * 60 * 1000)
	futureToleranceMill = uint64(5 * 60 * 1000)
)

type sampleRef struct {
	st *fieldState
	vh uint64
}

type batchStats struct {
	leftOut             map[leftOutReason]int
	resourcesOverflowed int
}

// batch builds the rows of one export call. It runs under the exporter lock,
// and the keys it adds to the day cache are stored only after the insert
// succeeds.
type batch struct {
	day           uint64
	limits        LimitsConfig
	alwaysInclude map[string]struct{}
	bodyLimits    *BodyJSONLimits
	state         *signalState
	class         *classification
	nowMillis     uint64

	rows         []row
	pending      map[uint64]cacheClass
	pendingCount [2]int
	// owners maps a pending key to the rows it writes, so the rows can be
	// dropped when a shared cache knows the key.
	owners  map[uint64][]int
	samples []sampleRef
	stats   batchStats
}

type resourceRef struct {
	hash  uint64
	state *resourceState
}

func (b *batch) known(k uint64) bool {
	if _, ok := b.pending[k]; ok {
		return true
	}
	return b.state.cache.has(k)
}

func (b *batch) room(class cacheClass) bool {
	return b.state.cache.room(class, b.state.inflight[class]+b.pendingCount[class]+1)
}

// classFor picks the exact part of the cache, or the reserve when the exact
// part is full.
func (b *batch) classFor() (cacheClass, bool) {
	if b.room(classExact) {
		return classExact, true
	}
	if b.room(classReserve) {
		return classReserve, true
	}
	return 0, false
}

func (b *batch) remember(k uint64, class cacheClass) {
	b.pending[k] = class
	b.pendingCount[class]++
}

func (b *batch) leaveOut(reason leftOutReason, n int) {
	if n > 0 {
		b.stats.leftOut[reason] += n
	}
}

// emit adds a row. owner is the cache key that writes the row, or 0 for a
// sample value, which the tracker deduplicates.
func (b *batch) emit(owner uint64, metricName string, p pair, resourceHash, attrsHash uint64, inHash bool, seen uint64) {
	if owner != 0 {
		b.owners[owner] = append(b.owners[owner], len(b.rows))
	}
	b.rows = append(b.rows, row{
		metricName:   metricName,
		p:            p,
		resourceHash: resourceHash,
		attrsHash:    attrsHash,
		inHash:       inHash,
		seenMillis:   seen,
	})
}

// seenMillis is the record time, or the fallback time, in milliseconds. A
// time of 0, older than a day, or more than 5 minutes in the future becomes
// the time of the batch, so a bad clock cannot keep a row past its TTL.
func (b *batch) seenMillis(ts, fallback pcommon.Timestamp) uint64 {
	t := uint64(ts)
	if t == 0 {
		t = uint64(fallback)
	}
	ms := t / 1e6
	if ms == 0 || ms+dayMillis < b.nowMillis || ms > b.nowMillis+futureToleranceMill {
		return b.nowMillis
	}
	return ms
}

// decide applies the value rules to one pair of logs or traces.
func (b *batch) decide(p pair, resource bool) mode {
	if p.typ != typeNumber {
		if p.str == "" {
			return modeSkip
		}
		if len(p.str) > b.limits.MaxValueBytes {
			b.leaveOut(reasonValueLength, 1)
			return modeSkip
		}
	}
	if _, ok := b.alwaysInclude[p.name]; ok {
		return modeInHash
	}
	fk := p.key()
	st := b.state.tracker.state(fk)
	if st == nil {
		b.leaveOut(reasonFieldPlaces, 1)
		return modeSkip
	}
	limit := int(b.limits.MaxRecordFieldValues)
	if resource {
		limit = int(b.limits.MaxResourceFieldValues)
	}
	vh := p.valueHash()
	_, seen := st.values[vh]

	if !st.over && !b.class.isOver(fk) {
		if seen || len(st.values) < limit {
			st.values[vh] = struct{}{}
			return modeInHash
		}
		// This value passes the limit: it is written first, and the field
		// leaves the hash on this collector for the rest of the day.
		st.values[vh] = struct{}{}
		st.over = true
		if resource {
			return modeOutside
		}
		b.samples = append(b.samples, sampleRef{st: st, vh: vh})
		return modeSample
	}

	if resource {
		if !seen && len(st.values) <= limit {
			st.values[vh] = struct{}{}
		}
		return modeOutside
	}
	if seen {
		return modeSkip
	}
	if len(st.values) <= limit {
		st.values[vh] = struct{}{}
		b.samples = append(b.samples, sampleRef{st: st, vh: vh})
		return modeSample
	}
	b.leaveOut(reasonSampleBudget, 1)
	return modeSkip
}

// resource writes the rows of a resource and returns its identity. The fields
// in the identity are written once per resource per day; the values of the
// resource fields outside the identity are written once per value.
func (b *batch) resource(attrs pcommon.Map, seen uint64) resourceRef {
	var identity, outside []pair
	for _, p := range appendAttrPairs(nil, contextResource, attrs) {
		switch b.decide(p, true) {
		case modeInHash:
			identity = append(identity, p)
		case modeOutside:
			outside = append(outside, p)
		}
	}
	rh := setHash(identity)
	if rk := resourceKey("", rh); !b.known(rk) {
		if class, ok := b.classFor(); ok {
			for _, p := range identity {
				b.emit(rk, "", p, rh, resourceAttrsHash, true, seen)
			}
			b.remember(rk, class)
		} else {
			b.leaveOut(reasonCacheFull, len(identity))
		}
	}
	for _, p := range outside {
		k := pairKey(rh, resourceAttrsHash, p)
		if b.known(k) {
			continue
		}
		if class, ok := b.classFor(); ok {
			b.emit(k, "", p, rh, resourceAttrsHash, false, seen)
			b.remember(k, class)
		} else {
			b.leaveOut(reasonCacheFull, 1)
		}
	}
	rs, ok := b.state.resources[rh]
	if !ok {
		rs = newResourceState()
		b.state.resources[rh] = rs
	}
	return resourceRef{hash: rh, state: rs}
}

// record writes the rows of one log record or span. recordPairs may enter the
// set hash; eventPairs never do.
func (b *batch) record(res resourceRef, recordPairs, eventPairs []pair, seen uint64) {
	rs := res.state
	var in, out, samples []pair
	for _, p := range recordPairs {
		switch b.decide(p, false) {
		case modeInHash:
			if rs.isDropped(p.key()) {
				out = append(out, p)
			} else {
				in = append(in, p)
			}
		case modeSample:
			samples = append(samples, p)
		}
	}
	for _, p := range eventPairs {
		switch b.decide(p, false) {
		case modeInHash:
			out = append(out, p)
		case modeSample:
			samples = append(samples, p)
		}
	}
	if rs.overflow {
		b.overflow(res, in, out, samples, seen)
		return
	}

	ah := setHash(in)
	k := setKey(res.hash, ah)
	if !b.known(k) {
		if rs.stepSets >= b.limits.MaxSetsPerResource/2 {
			rs.observe(in)
		}
		for rs.stepSets >= b.limits.MaxSetsPerResource || rs.stepPairs >= b.limits.MaxOutsidePairsPerResource {
			if !rs.stepUp() {
				rs.overflow = true
				break
			}
			in, out = moveDropped(rs, in, out)
			ah = setHash(in)
			k = setKey(res.hash, ah)
			if b.known(k) {
				break
			}
		}
		if rs.overflow {
			b.overflow(res, in, out, samples, seen)
			return
		}
		if !b.known(k) {
			if !b.room(classExact) {
				b.overflow(res, in, out, samples, seen)
				return
			}
			for _, p := range in {
				b.emit(k, "", p, res.hash, ah, true, seen)
			}
			b.remember(k, classExact)
			rs.stepSets++
		}
	}
	for _, p := range out {
		pk := pairKey(res.hash, ah, p)
		if b.known(pk) {
			continue
		}
		// A known set can keep getting new pairs outside its hash, such as a
		// new path in a coarse set. Past the limit of the step, they go into
		// the overflow set, which holds each pair of the resource once.
		if !b.room(classExact) || rs.stepPairs >= b.limits.MaxOutsidePairsPerResource {
			b.overflowPair(res, p, seen)
			continue
		}
		b.emit(pk, "", p, res.hash, ah, false, seen)
		b.remember(pk, classExact)
		rs.stepPairs++
	}
	for _, p := range samples {
		b.emit(0, "", p, res.hash, ah, false, seen)
	}
}

// overflow writes a record into the overflow set of its resource: every pair
// once per day, outside the hash.
func (b *batch) overflow(res resourceRef, in, out, samples []pair, seen uint64) {
	for _, p := range in {
		b.overflowPair(res, p, seen)
	}
	for _, p := range out {
		b.overflowPair(res, p, seen)
	}
	for _, p := range samples {
		b.emit(0, "", p, res.hash, overflowAttrsHash, false, seen)
	}
}

func (b *batch) overflowPair(res resourceRef, p pair, seen uint64) {
	if !res.state.overflowCounted {
		res.state.overflowCounted = true
		b.stats.resourcesOverflowed++
	}
	k := pairKey(res.hash, overflowAttrsHash, p)
	if b.known(k) {
		return
	}
	class, ok := b.classFor()
	if !ok {
		b.leaveOut(reasonCacheFull, 1)
		return
	}
	b.emit(k, "", p, res.hash, overflowAttrsHash, false, seen)
	b.remember(k, class)
}

func moveDropped(rs *resourceState, in, out []pair) ([]pair, []pair) {
	kept := in[:0:0]
	for _, p := range in {
		if rs.isDropped(p.key()) {
			out = append(out, p)
		} else {
			kept = append(kept, p)
		}
	}
	return kept, out
}

// appendAttrPairs converts attributes to pairs. Nested maps become dotted
// names; slices are kept as their JSON string.
func appendAttrPairs(dst []pair, ctx fieldContext, m pcommon.Map) []pair {
	return appendAttrPairsWithPrefix(dst, ctx, m, "")
}

func appendAttrPairsWithPrefix(dst []pair, ctx fieldContext, m pcommon.Map, prefix string) []pair {
	out := dst
	m.Range(func(k string, v pcommon.Value) bool {
		name := k
		if prefix != "" {
			name = prefix + "." + k
		}
		switch v.Type() {
		case pcommon.ValueTypeStr:
			out = append(out, pair{ctx: ctx, name: name, typ: typeString, str: v.Str()})
		case pcommon.ValueTypeInt:
			out = append(out, pair{ctx: ctx, name: name, typ: typeNumber, num: float64(v.Int())})
		case pcommon.ValueTypeDouble:
			if f := v.Double(); !math.IsNaN(f) && !math.IsInf(f, 0) {
				out = append(out, pair{ctx: ctx, name: name, typ: typeNumber, num: f})
			}
		case pcommon.ValueTypeBool:
			out = append(out, pair{ctx: ctx, name: name, typ: typeBool, str: strconv.FormatBool(v.Bool())})
		case pcommon.ValueTypeMap:
			out = appendAttrPairsWithPrefix(out, ctx, v.Map(), name)
		case pcommon.ValueTypeSlice:
			out = append(out, pair{ctx: ctx, name: name, typ: typeString, str: v.AsString()})
		}
		return true
	})
	return out
}

func stringPair(ctx fieldContext, name, value string) pair {
	return pair{ctx: ctx, name: name, typ: typeString, str: value}
}

func numberPair(ctx fieldContext, name string, value float64) pair {
	return pair{ctx: ctx, name: name, typ: typeNumber, num: value}
}

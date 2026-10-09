package fieldvalues

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
)

type leftOutReason string

const (
	reasonValueLength  leftOutReason = "value_length"
	reasonFieldPlaces  leftOutReason = "field_places"
	reasonSampleBudget leftOutReason = "sample_budget"
	reasonCacheFull    leftOutReason = "cache_full"
	reasonTrackerFull  leftOutReason = "tracker_full"
)

// mode is what a pair of a record does.
type mode uint8

const (
	modeSkip    mode = iota
	modeInHash       // part of the set hash, or of the resource identity
	modeOutside      // outside the hash, written once per set (or resource) and value per window
	modeSample       // outside the hash, written once per value per day
)

const (
	dayMillis             = uint64(24 * 60 * 60 * 1000)
	futureToleranceMillis = uint64(5 * 60 * 1000)
)

type sampleRef struct {
	st *fieldState
	vh uint64
}

type batchStats struct {
	leftOut             map[leftOutReason]int
	resourcesOverflowed int
	resourcesUntracked  int
	keysWrittenAhead    int
}

// batch builds the rows of one export call. It runs under the writer lock,
// and the keys it adds to the key cache are stored only after the insert
// succeeds.
type batch struct {
	window        Window
	dayStart      uint64
	limits        LimitsConfig
	alwaysInclude map[string]struct{}
	state         *signalState
	class         *classification
	nowMillis     uint64
	// preWriteMillis is the length of the pre-write window, or 0.
	preWriteMillis uint64

	rows         []row
	pending      map[uint64]cacheClass
	pendingCount [2]int
	// pendingAhead holds the keys written ahead for the next window.
	pendingAhead map[uint64]struct{}

	// owners maps a pending key to the rows it writes, so the rows can be
	// dropped when a shared cache knows the key. Index 0 is the window, 1 the
	// next window.
	owners  [2]map[uint64][]int
	samples []sampleRef
	stats   batchStats

	// in, out and sampled are the buffers of record and resource, reused for
	// each record. refs holds the resources of the input.
	in      []pair
	out     []pair
	sampled []pair
	refs    []resourceRef
	// lastResource and lastLink are the resource and the link of the last
	// metric point, and keysChecked holds the metrics and resource field
	// names whose key rows this batch checked.
	lastResource int
	lastLink     uint64
	keysChecked  map[uint64]struct{}
}

func newBatchBuffers() *batch {
	return &batch{
		pending:      make(map[uint64]cacheClass),
		pendingAhead: make(map[uint64]struct{}),
		keysChecked:  make(map[uint64]struct{}),
		lastResource: -1,
		owners:       [2]map[uint64][]int{make(map[uint64][]int), make(map[uint64][]int)},
		stats:        batchStats{leftOut: make(map[leftOutReason]int)},
	}
}

// reset empties the batch for the next push. Rows and pairs are cleared, so
// a pooled batch holds no strings of an earlier push.
func (b *batch) reset() {
	clear(b.rows)
	b.rows = b.rows[:0]
	clear(b.pending)
	b.pendingCount = [2]int{}
	clear(b.pendingAhead)
	clear(b.owners[0])
	clear(b.owners[1])
	clear(b.samples)
	b.samples = b.samples[:0]
	clear(b.stats.leftOut)
	b.stats = batchStats{leftOut: b.stats.leftOut}
	clear(b.in[:cap(b.in)])
	clear(b.out[:cap(b.out)])
	clear(b.sampled[:cap(b.sampled)])
	clear(b.refs)
	b.refs = b.refs[:0]
	b.lastResource, b.lastLink = -1, 0
	clear(b.keysChecked)
	b.state, b.class = nil, nil
}

type resourceRef struct {
	hash  uint64
	state *resourceState
	ready bool
}

// lookup reports whether a key was written in the window, and whether its
// rows are now due to be written ahead for the next window. In the last
// preWriteMillis of the window, pkg/timebucketedset makes each known key due
// from a time set by its hash, so the next window fills over this time and not
// at its start.
func (b *batch) lookup(k uint64) (known, ahead bool) {
	if _, ok := b.pending[k]; ok {
		return true, false
	}
	write, next := b.state.cache.plan(k, b.window.Start, b.nowMillis)
	if write {
		return false, false
	}
	if !next {
		return true, false
	}
	if _, ok := b.pendingAhead[k]; ok {
		return true, false
	}
	return true, b.state.cache.roomNext(b.state.inflightAhead + len(b.pendingAhead) + 1)
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
func (b *batch) emit(owner uint64, metricName string, p *pair, resourceHash, attrsHash uint64, inHash bool, seen uint64) {
	if owner != 0 {
		b.owners[0][owner] = append(b.owners[0][owner], len(b.rows))
	}
	b.rows = append(b.rows, row{
		metricName:   metricName,
		p:            *p,
		resourceHash: resourceHash,
		attrsHash:    attrsHash,
		inHash:       inHash,
		seenMillis:   seen,
	})
}

// emitAhead writes the rows of a key for the next window, at its start.
func (b *batch) emitAhead(k uint64, metricName string, pairs []pair, resourceHash, attrsHash uint64, inHash bool) {
	for i := range pairs {
		b.owners[1][k] = append(b.owners[1][k], len(b.rows))
		b.rows = append(b.rows, row{
			metricName:   metricName,
			p:            pairs[i],
			resourceHash: resourceHash,
			attrsHash:    attrsHash,
			inHash:       inHash,
			seenMillis:   b.window.End,
		})
	}
	b.pendingAhead[k] = struct{}{}
	b.stats.keysWrittenAhead++
}

// seenMillis is the record time, or the fallback time, in milliseconds. A
// time of 0 or more than 5 minutes in the future becomes the time of the
// batch. A time before the window becomes the start of the window: the key is
// cached for this window, so its row must be in it.
func (b *batch) seenMillis(ts, fallback pcommon.Timestamp) uint64 {
	t := uint64(ts)
	if t == 0 {
		t = uint64(fallback)
	}
	ms := t / 1e6
	switch {
	case ms == 0 || ms > b.nowMillis+futureToleranceMillis:
		ms = b.nowMillis
	case ms < b.window.Start:
		ms = b.window.Start
	}
	return min(ms, b.window.End-1)
}

// decide applies the value rules to one pair of logs or traces.
func (b *batch) decide(p *pair, resource bool) mode {
	if p.typ != typeNumber {
		if p.str == "" {
			return modeSkip
		}
		if len(p.str) > b.limits.MaxValueBytes {
			b.leaveOut(reasonValueLength, 1)
			return modeSkip
		}
	}
	if len(b.alwaysInclude) > 0 {
		if _, ok := b.alwaysInclude[p.name]; ok {
			return modeInHash
		}
	}
	tr := b.state.tracker
	st := tr.state(p.field)
	if st == nil {
		b.leaveOut(reasonFieldPlaces, 1)
		return modeSkip
	}
	limit := int(b.limits.MaxRecordFieldValues)
	if resource {
		limit = int(b.limits.MaxResourceFieldValues)
	}

	if !st.over && !b.class.isOver(p.field) {
		if st.values.has(p.vh) {
			return modeInHash
		}
		if st.values.len() < limit {
			// When the tracker is full, the value is not counted. The reads of
			// field_values_daily and the coarse sets still bound the field.
			tr.add(st, p.vh)
			return modeInHash
		}
		// This value passes the limit: it is written first, and the field
		// leaves the hash on this collector for the rest of the day.
		st.over = true
	}
	if resource {
		if !st.spent {
			tr.spend(st)
		}
		return modeOutside
	}
	return b.sample(st, p.vh)
}

// sample takes a value of a field over the limit into the daily sample, up to
// the limit plus one values per day.
func (b *batch) sample(st *fieldState, vh uint64) mode {
	tr := b.state.tracker
	switch {
	case st.spent:
		b.leaveOut(reasonSampleBudget, 1)
		return modeSkip
	case st.values.has(vh):
		return modeSkip
	case st.values.len() > int(b.limits.MaxRecordFieldValues):
		tr.spend(st)
		b.leaveOut(reasonSampleBudget, 1)
		return modeSkip
	case !b.sampleDue(st):
		return modeSkip
	case !tr.add(st, vh):
		b.leaveOut(reasonTrackerFull, 1)
		return modeSkip
	}
	b.samples = append(b.samples, sampleRef{st: st, vh: vh})
	return modeSample
}

// sampleDue spreads the daily sample over the first preWriteMillis of the UTC
// day: until then, the values a field holds may not pass the share of its
// budget that the elapsed time of the day gives.
func (b *batch) sampleDue(st *fieldState) bool {
	elapsed := b.nowMillis - b.dayStart
	if b.preWriteMillis == 0 || elapsed >= b.preWriteMillis {
		return true
	}
	budget := b.limits.MaxRecordFieldValues + 1
	return uint64(st.values.len()+1)*b.preWriteMillis <= budget*elapsed
}

// resource writes the rows of a resource and returns its identity. The fields
// in the identity are written once per resource per window; the values of the
// resource fields outside the identity are written once per value.
func (b *batch) resource(pairs []pair, seen uint64) resourceRef {
	identity, outside := b.in[:0], b.out[:0]
	for i := range pairs {
		switch b.decide(&pairs[i], true) {
		case modeInHash:
			identity = append(identity, pairs[i])
		case modeOutside:
			outside = append(outside, pairs[i])
		}
	}
	b.in, b.out = identity, outside
	rh := setHash(identity)
	rk := resourceKey(emptyNameHash, rh)
	if known, ahead := b.lookup(rk); !known {
		if class, ok := b.classFor(); ok {
			for i := range identity {
				b.emit(rk, "", &identity[i], rh, resourceAttrsHash, true, seen)
			}
			b.remember(rk, class)
		} else {
			b.leaveOut(reasonCacheFull, len(identity))
		}
	} else if ahead {
		b.emitAhead(rk, "", identity, rh, resourceAttrsHash, true)
	}
	for i := range outside {
		k := pairKey(rh, resourceAttrsHash, &outside[i])
		if known, ahead := b.lookup(k); known {
			if ahead {
				b.emitAhead(k, "", outside[i:i+1], rh, resourceAttrsHash, false)
			}
			continue
		}
		if class, ok := b.classFor(); ok {
			b.emit(k, "", &outside[i], rh, resourceAttrsHash, false, seen)
			b.remember(k, class)
		} else {
			b.leaveOut(reasonCacheFull, 1)
		}
	}
	rs := b.state.resources.get(rh)
	if rs == nil {
		b.stats.resourcesUntracked++
	}
	return resourceRef{hash: rh, state: rs, ready: true}
}

// record writes the rows of one log record or span. recordPairs may enter the
// set hash; eventPairs never do. A resource without a state, because the
// budget is full, writes into its overflow set.
func (b *batch) record(res resourceRef, recordPairs, eventPairs []pair, seen uint64) {
	rs := res.state
	in, out, samples := b.in[:0], b.out[:0], b.sampled[:0]
	for i := range recordPairs {
		p := &recordPairs[i]
		switch b.decide(p, false) {
		case modeInHash:
			if rs != nil && rs.isDropped(p.field) {
				out = append(out, *p)
			} else {
				in = append(in, *p)
			}
		case modeSample:
			samples = append(samples, *p)
		}
	}
	for i := range eventPairs {
		p := &eventPairs[i]
		switch b.decide(p, false) {
		case modeInHash:
			out = append(out, *p)
		case modeSample:
			samples = append(samples, *p)
		}
	}
	b.in, b.out, b.sampled = in, out, samples
	if rs == nil || rs.overflow {
		b.overflow(res, in, out, samples, seen)
		return
	}

	ah := setHash(in)
	k := setKey(res.hash, ah)
	known, ahead := b.lookup(k)
	if !known {
		if rs.stepSets >= b.limits.MaxSetsPerResource/2 {
			rs.observe(in, b.state.resources)
		}
		for rs.stepSets >= b.limits.MaxSetsPerResource || rs.stepPairs >= b.limits.MaxOutsidePairsPerResource {
			if !rs.stepUp() {
				rs.overflow = true
				break
			}
			in, out = moveDropped(rs, in, out)
			b.in, b.out = in, out
			ah = setHash(in)
			k = setKey(res.hash, ah)
			if known, ahead = b.lookup(k); known {
				break
			}
		}
		if rs.overflow {
			b.overflow(res, in, out, samples, seen)
			return
		}
		if !known {
			if !b.room(classExact) {
				b.overflow(res, in, out, samples, seen)
				return
			}
			for i := range in {
				b.emit(k, "", &in[i], res.hash, ah, true, seen)
			}
			b.remember(k, classExact)
			rs.stepSets++
		}
	}
	if known && ahead {
		b.emitAhead(k, "", in, res.hash, ah, true)
	}
	for i := range out {
		pk := pairKey(res.hash, ah, &out[i])
		if known, ahead := b.lookup(pk); known {
			if ahead {
				b.emitAhead(pk, "", out[i:i+1], res.hash, ah, false)
			}
			continue
		}
		// A known set can keep getting new pairs outside its hash, such as a
		// new path in a coarse set. Past the limit of the step, they go into
		// the overflow set, which holds each pair of the resource once.
		if !b.room(classExact) || rs.stepPairs >= b.limits.MaxOutsidePairsPerResource {
			b.overflowPair(res, out[i:i+1], seen)
			continue
		}
		b.emit(pk, "", &out[i], res.hash, ah, false, seen)
		b.remember(pk, classExact)
		rs.stepPairs++
	}
	for i := range samples {
		b.emit(0, "", &samples[i], res.hash, ah, false, seen)
	}
}

// overflow writes a record into the overflow set of its resource: every pair
// once per window, outside the hash.
func (b *batch) overflow(res resourceRef, in, out, samples []pair, seen uint64) {
	for i := range in {
		b.overflowPair(res, in[i:i+1], seen)
	}
	for i := range out {
		b.overflowPair(res, out[i:i+1], seen)
	}
	for i := range samples {
		b.emit(0, "", &samples[i], res.hash, overflowAttrsHash, false, seen)
	}
}

// overflowPair writes the one pair of p into the overflow set.
func (b *batch) overflowPair(res resourceRef, p []pair, seen uint64) {
	if res.state != nil && !res.state.overflowCounted {
		res.state.overflowCounted = true
		b.stats.resourcesOverflowed++
	}
	k := pairKey(res.hash, overflowAttrsHash, &p[0])
	if known, ahead := b.lookup(k); known {
		if ahead {
			b.emitAhead(k, "", p, res.hash, overflowAttrsHash, false)
		}
		return
	}
	class, ok := b.classFor()
	if !ok {
		b.leaveOut(reasonCacheFull, 1)
		return
	}
	b.emit(k, "", &p[0], res.hash, overflowAttrsHash, false, seen)
	b.remember(k, class)
}

// moveDropped moves the pairs of dropped fields from in to out, in place.
func moveDropped(rs *resourceState, in, out []pair) ([]pair, []pair) {
	kept := in[:0]
	for i := range in {
		if rs.isDropped(in[i].field) {
			out = append(out, in[i])
		} else {
			kept = append(kept, in[i])
		}
	}
	return kept, out
}

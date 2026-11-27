package fieldvalues

import "math"

const (
	// maxKnownKeys bounds the memory of the keys with a row of today. Past
	// it, a key is written again in each flush in which it is seen. No key
	// is left out.
	maxKnownKeys = 1 << 20
	// maxPendingKeys bounds the keys that wait for an insert. A flush starts
	// at keysFlushAt, so only inserts that fail for a long time fill it. A
	// key past it is counted in notQueued and waits for its next record; for
	// a label of a metric, that is the next day.
	maxPendingKeys = 1 << 22
	keysFlushAt    = 1 << 16
	// keysInsertRows is the largest insert of keys.
	keysInsertRows = 1 << 17
	// recentKeys is the size of the table that answers most checks of a key
	// with one compare.
	recentKeys = 1 << 15
)

// fieldKey is one row of field_keys_v2.
type fieldKey struct {
	metric   string
	ctx      fieldContext
	name     string
	dataType string
}

// keyDataType is the data type of the key of a pair: the type with which the
// exporter of the signal stores the value.
func (p *pair) keyDataType() string {
	switch {
	case p.array != arrayNone:
		return p.array.String()
	case p.typ == typeBool:
		return "bool"
	case p.typ == typeString:
		return "string"
	case p.isInt:
		return "int64"
	}
	return "float64"
}

// fieldKeys holds the field keys of one signal. The key list must be complete:
// a key is recorded before any rule of the values can leave its pair out, and
// it counts as written only after an insert that the server confirmed.
type fieldKeys struct {
	day uint64
	// known holds the keys with a confirmed insert on day, and recent a part
	// of them.
	known  map[uint64]struct{}
	recent [recentKeys]uint64
	// pending holds the keys seen with no confirmed insert on day.
	pending map[uint64]fieldKey
	// notQueued counts the keys that pending had no room for.
	notQueued int
}

func newFieldKeys() *fieldKeys {
	return &fieldKeys{known: make(map[uint64]struct{}), pending: make(map[uint64]fieldKey)}
}

// rotate starts a UTC day: each key is written again, which moves its
// last_seen.
func (ks *fieldKeys) rotate(day uint64) {
	if day == ks.day {
		return
	}
	ks.day = day
	clear(ks.known)
	ks.recent = [recentKeys]uint64{}
}

// see records the key of a pair. A label of a metric has the metric name and
// its hash.
func (ks *fieldKeys) see(p *pair, metricNameHash uint64, metricName string) {
	// The id covers what makes the row: the field and its data type.
	dataType := uint64(p.typ) << 1
	switch {
	case p.array != arrayNone:
		dataType = uint64(p.array) << 8
	case p.isInt:
		dataType |= 1
	}
	id := mix64(uint64(p.field) ^ metricNameHash*0x9e3779b97f4a7c15 ^ dataType<<8 ^ 'F')
	// 0 marks an empty place of recent.
	id |= 1 << 63
	slot := &ks.recent[id%recentKeys]
	if *slot == id {
		return
	}
	if _, ok := ks.known[id]; ok {
		*slot = id
		return
	}
	if _, ok := ks.pending[id]; ok {
		return
	}
	if len(ks.pending) >= maxPendingKeys {
		ks.notQueued++
		return
	}
	ks.pending[id] = fieldKey{metric: metricName, ctx: p.ctx, name: p.name, dataType: p.keyDataType()}
}

// take returns the keys that wait for an insert. They stay pending until
// confirm.
func (ks *fieldKeys) take() (ids []uint64, keys []fieldKey) {
	if len(ks.pending) == 0 {
		return nil, nil
	}
	ids = make([]uint64, 0, len(ks.pending))
	keys = make([]fieldKey, 0, len(ks.pending))
	for id, k := range ks.pending {
		ids = append(ids, id)
		keys = append(keys, k)
	}
	return ids, keys
}

// confirm marks the keys of an insert that the server confirmed on day.
func (ks *fieldKeys) confirm(ids []uint64, day uint64) {
	for _, id := range ids {
		delete(ks.pending, id)
		if day == ks.day && len(ks.known) < maxKnownKeys {
			ks.known[id] = struct{}{}
		}
	}
}

// hasValue reports whether a number can be a value. NaN and the infinities
// have a key and no value.
func hasValue(f float64) bool {
	return !math.IsNaN(f) && !math.IsInf(f, 0)
}

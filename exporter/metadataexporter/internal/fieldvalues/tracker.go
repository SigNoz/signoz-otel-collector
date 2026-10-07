package fieldvalues

// budget is the memory that the value tracker and the resource states of one
// signal may use. The window cache has a fixed size of its own.
type budget struct {
	used  int
	limit int
}

func (b *budget) take(n int) bool {
	if b.used+n > b.limit {
		return false
	}
	b.used += n
	return true
}

func (b *budget) give(n int) {
	b.used -= n
}

// fieldState holds the hashes of the values of one field that this collector
// wrote today, up to the limit plus one.
type fieldState struct {
	values valueSet
	// over is set when this collector saw more values than the limit today.
	over bool
	// spent is set when the values are no longer needed: a resource field over
	// its limit, or a record field whose daily sample is full. Its values are
	// freed.
	spent bool
}

// fieldStateBytes is the memory of a field state without its values.
const fieldStateBytes = 96

// tracker is the unique value tracker of one signal for the UTC day. It counts
// values per field, decides the daily budget of high-cardinality fields, and
// owns the field places.
type tracker struct {
	maxFields int
	fields    map[fieldID]*fieldState
	budget    *budget
	bytes     int
}

func newTracker(maxFields int, b *budget) *tracker {
	return &tracker{maxFields: maxFields, fields: make(map[fieldID]*fieldState), budget: b}
}

// reset starts a new day. The known fields get their places first, so a burst
// of new keys cannot take them.
func (t *tracker) reset(known []fieldID) {
	t.budget.give(t.bytes)
	t.bytes = 0
	t.fields = make(map[fieldID]*fieldState, len(known))
	t.place(known)
}

func (t *tracker) place(known []fieldID) {
	for _, f := range known {
		if len(t.fields) >= t.maxFields {
			return
		}
		if _, ok := t.fields[f]; !ok {
			t.newState(f)
		}
	}
}

// state returns the state of a field, or nil when the field has no place.
func (t *tracker) state(f fieldID) *fieldState {
	if st, ok := t.fields[f]; ok {
		return st
	}
	if len(t.fields) >= t.maxFields {
		return nil
	}
	return t.newState(f)
}

func (t *tracker) newState(f fieldID) *fieldState {
	st := &fieldState{}
	t.fields[f] = st
	t.budget.used += fieldStateBytes
	t.bytes += fieldStateBytes
	return st
}

// add records a value. It returns false when the set must grow and the budget
// has no room.
func (t *tracker) add(st *fieldState, v uint64) bool {
	if size, grow := st.values.growth(); grow {
		delta := (size - len(st.values.slots)) * slotBytes
		if !t.budget.take(delta) {
			return false
		}
		t.bytes += delta
		st.values.resize(size)
	}
	st.values.add(v)
	return true
}

func (t *tracker) spend(st *fieldState) {
	freed := len(st.values.slots) * slotBytes
	t.budget.give(freed)
	t.bytes -= freed
	st.values = valueSet{}
	st.spent = true
}

const minValueSlots = 8

// valueSet is a set of value hashes in one table with linear probing. It
// costs 8 to 16 bytes per value, a quarter to a half of a Go map.
type valueSet struct {
	slots []uint64
	n     int
}

func nonZero(v uint64) uint64 {
	if v == 0 {
		return 1
	}
	return v
}

func (s *valueSet) len() int {
	return s.n
}

func (s *valueSet) has(v uint64) bool {
	if len(s.slots) == 0 {
		return false
	}
	v = nonZero(v)
	mask := uint64(len(s.slots) - 1)
	for i := v & mask; ; i = (i + 1) & mask {
		switch s.slots[i] {
		case v:
			return true
		case 0:
			return false
		}
	}
}

// growth returns the size the table needs before one more value, and whether
// it must grow. The table stays at most 75% full.
func (s *valueSet) growth() (int, bool) {
	if (s.n+1)*4 <= len(s.slots)*3 {
		return len(s.slots), false
	}
	return max(len(s.slots)*2, minValueSlots), true
}

func (s *valueSet) resize(size int) {
	old := s.slots
	s.slots = make([]uint64, size)
	s.n = 0
	for _, v := range old {
		if v != 0 {
			s.add(v)
		}
	}
}

// add inserts v into a table with room.
func (s *valueSet) add(v uint64) {
	v = nonZero(v)
	mask := uint64(len(s.slots) - 1)
	for i := v & mask; ; i = (i + 1) & mask {
		switch s.slots[i] {
		case v:
			return
		case 0:
			s.slots[i] = v
			s.n++
			return
		}
	}
}

// remove deletes v and moves later values of its run back, so that lookups
// need no markers of deleted values.
func (s *valueSet) remove(v uint64) {
	if len(s.slots) == 0 {
		return
	}
	v = nonZero(v)
	mask := uint64(len(s.slots) - 1)
	i := v & mask
	for s.slots[i] != v {
		if s.slots[i] == 0 {
			return
		}
		i = (i + 1) & mask
	}
	for j := (i + 1) & mask; s.slots[j] != 0; j = (j + 1) & mask {
		home := s.slots[j] & mask
		// The value at j stays when its home is cyclically in (i, j].
		if (i < j && i < home && home <= j) || (i > j && (home > i || home <= j)) {
			continue
		}
		s.slots[i] = s.slots[j]
		i = j
	}
	s.slots[i] = 0
	s.n--
}

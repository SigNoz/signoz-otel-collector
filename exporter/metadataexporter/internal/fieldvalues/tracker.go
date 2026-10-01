package fieldvalues

// fieldState holds the hashes of the values of one field that this collector
// wrote today, up to the limit plus one.
type fieldState struct {
	values map[uint64]struct{}
	// over is set when this collector saw more values than the limit today.
	over bool
}

// tracker is the unique value tracker of one signal for the UTC day. It counts
// values per field, decides the daily budget of high-cardinality fields, and
// owns the field places.
type tracker struct {
	maxFields int
	fields    map[fieldKey]*fieldState
}

func newTracker(maxFields int) *tracker {
	return &tracker{maxFields: maxFields, fields: make(map[fieldKey]*fieldState)}
}

// reset starts a new day. The known fields get their places first, so a burst
// of new keys cannot take them.
func (t *tracker) reset(known []fieldKey) {
	t.fields = make(map[fieldKey]*fieldState, len(known))
	t.place(known)
}

func (t *tracker) place(known []fieldKey) {
	for _, fk := range known {
		if len(t.fields) >= t.maxFields {
			return
		}
		if _, ok := t.fields[fk]; !ok {
			t.fields[fk] = &fieldState{values: make(map[uint64]struct{})}
		}
	}
}

// state returns the state of a field, or nil when the field has no place.
func (t *tracker) state(fk fieldKey) *fieldState {
	if st, ok := t.fields[fk]; ok {
		return st
	}
	if len(t.fields) >= t.maxFields {
		return nil
	}
	st := &fieldState{values: make(map[uint64]struct{})}
	t.fields[fk] = st
	return st
}

package fieldvalues

// resourceState is the state of one resource for the window on this
// collector: its coarse steps and whether it uses its overflow set.
type resourceState struct {
	// dropped are the fields that left the hash of this resource.
	dropped map[fieldID]struct{}
	// stepSets and stepPairs count the new set keys and the new keys of pairs
	// outside the hash in the current step. Each step has its own limit.
	stepSets  int
	stepPairs int
	// sketches count the values of each field, from half the set limit on.
	sketches map[fieldID]*sketch
	overflow bool
	// overflowCounted is set once the resource first wrote into its overflow
	// set in the window, so telemetry counts each resource once.
	overflowCounted bool
}

const (
	// resourceStateBytes and sketchBytes are the memory of a resource state
	// with its map entry, and of a sketch with its map entry.
	resourceStateBytes = 128
	sketchBytes        = sketchExactValues*8 + sketchRegisters + 64
)

// resourceTable holds the resource states of the window within the budget.
type resourceTable struct {
	states map[uint64]*resourceState
	budget *budget
	bytes  int
}

func newResourceTable(b *budget) *resourceTable {
	return &resourceTable{states: make(map[uint64]*resourceState), budget: b}
}

func (r *resourceTable) reset() {
	r.budget.give(r.bytes)
	r.bytes = 0
	r.states = make(map[uint64]*resourceState)
}

// get returns the state of a resource, or nil when the budget is full.
func (r *resourceTable) get(h uint64) *resourceState {
	if rs, ok := r.states[h]; ok {
		return rs
	}
	if !r.take(resourceStateBytes) {
		return nil
	}
	rs := &resourceState{}
	r.states[h] = rs
	return rs
}

func (r *resourceTable) take(n int) bool {
	if !r.budget.take(n) {
		return false
	}
	r.bytes += n
	return true
}

func (rs *resourceState) isDropped(f fieldID) bool {
	_, ok := rs.dropped[f]
	return ok
}

// observe adds the values of a new set to the sketches. A field without a
// sketch when the budget is full is not counted, so it cannot be dropped.
func (rs *resourceState) observe(pairs []pair, table *resourceTable) {
	if rs.sketches == nil {
		rs.sketches = make(map[fieldID]*sketch)
	}
	for i := range pairs {
		p := &pairs[i]
		s, ok := rs.sketches[p.field]
		if !ok {
			if !table.take(sketchBytes) {
				continue
			}
			s = &sketch{}
			rs.sketches[p.field] = s
		}
		s.add(p.vh)
	}
}

// stepUp drops the field with the most values from the hash of this resource
// and starts a new step. It returns false when no field is left to drop.
func (rs *resourceState) stepUp() bool {
	var best fieldID
	bestEstimate := -1.0
	for f, s := range rs.sketches {
		if rs.isDropped(f) {
			continue
		}
		e := s.estimate()
		if e > bestEstimate || (e == bestEstimate && f < best) {
			best, bestEstimate = f, e
		}
	}
	if bestEstimate < 0 {
		return false
	}
	if rs.dropped == nil {
		rs.dropped = make(map[fieldID]struct{})
	}
	rs.dropped[best] = struct{}{}
	delete(rs.sketches, best)
	rs.stepSets, rs.stepPairs = 0, 0
	return true
}

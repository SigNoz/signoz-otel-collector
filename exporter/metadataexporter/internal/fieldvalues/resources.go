package fieldvalues

// resourceState is the state of one resource for the UTC day on this
// collector: its coarse steps and whether it uses its overflow set.
type resourceState struct {
	// dropped are the fields that left the hash of this resource.
	dropped map[fieldKey]struct{}
	// stepSets and stepPairs count the new set keys and the new keys of pairs
	// outside the hash in the current step. Each step has its own limit.
	stepSets  int
	stepPairs int
	// sketches count the values of each field, from half the set limit on.
	sketches map[fieldKey]*sketch
	overflow bool
	// overflowCounted is set once the resource first wrote into its overflow
	// set today, so telemetry counts each resource once.
	overflowCounted bool
}

func newResourceState() *resourceState {
	return &resourceState{dropped: make(map[fieldKey]struct{})}
}

func (r *resourceState) isDropped(fk fieldKey) bool {
	_, ok := r.dropped[fk]
	return ok
}

func (r *resourceState) observe(pairs []pair) {
	if r.sketches == nil {
		r.sketches = make(map[fieldKey]*sketch)
	}
	for _, p := range pairs {
		fk := p.key()
		s, ok := r.sketches[fk]
		if !ok {
			s = &sketch{}
			r.sketches[fk] = s
		}
		s.add(p.valueHash())
	}
}

// stepUp drops the field with the most values from the hash of this resource
// and starts a new step. It returns false when no field is left to drop.
func (r *resourceState) stepUp() bool {
	var best fieldKey
	bestEstimate := -1.0
	for fk, s := range r.sketches {
		if r.isDropped(fk) {
			continue
		}
		e := s.estimate()
		if e > bestEstimate || (e == bestEstimate && lessFieldKey(fk, best)) {
			best, bestEstimate = fk, e
		}
	}
	if bestEstimate < 0 {
		return false
	}
	r.dropped[best] = struct{}{}
	delete(r.sketches, best)
	r.stepSets, r.stepPairs = 0, 0
	return true
}

func lessFieldKey(a, b fieldKey) bool {
	if a.ctx != b.ctx {
		return a.ctx < b.ctx
	}
	return a.name < b.name
}

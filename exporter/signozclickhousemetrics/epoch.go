package signozclickhousemetrics

import (
	"sync"
	"time"
)

// epochNormalizer assigns an epoch (normalized start timestamp, in ms) to
// every cumulative monotonic sample. The invariant it maintains is the one the
// storage layer depends on: within a single (series, epoch) the value sequence
// is non-decreasing, so per-epoch min/max in the aggregation tables are exact
// first/last observations and rate/increase can be computed reset-exactly at
// any step interval.
//
// The output is always either a VALIDATED WIRE VALUE or 0 — the normalizer
// never invents timestamps. This is what makes it safe when the same series is
// load-balanced across collector replicas with no stickiness: a wire start is
// a property of the data, so every replica that trusts it emits the same
// epoch, while every distrust decision emits 0. Diverging per-replica epochs
// for one series would be read as parallel writers and double-count; emitting
// 0 degrades that series to the legacy negative-diff semantics instead.
//
// Epoch 0 means "unknown": the querier applies the legacy heuristic to those
// rows, so every failure mode here (state loss, churny sources, broken
// monotonicity, replica disagreement during a transient) reproduces today's
// behavior, never fabricated spikes.
//
// Rules, in order of trust:
//   - A valid wire start (0 < start <= t) is adopted when it first appears,
//     when the value drops together with a start change (a genuine reset),
//     or when the start changes after being stable for >=
//     epochStableThreshold points (a restart whose counter regrew past the
//     previous value before the next export — the reset that value-based
//     detection can never see).
//   - A changed start WITHOUT a value drop on an unstable series distrusts
//     the series to epoch 0 (churn guard): some spec-violating SDKs advance
//     start_time on every export, and honoring that would shred a cumulative
//     series into per-point epochs and massively overcount. A distrusted
//     series re-earns its epoch once the wire start holds still.
//   - A value drop WITHOUT a start change means the source's start time
//     cannot be trusted to delimit monotone runs: distrust to 0. (No
//     synthesis: a synthesized timestamp would differ per replica and split
//     the series into parallel "writers".)
type epochNormalizer struct {
	shards [epochShardCount]epochShard
	// now is swappable for tests
	now func() time.Time
}

type epochShard struct {
	mu sync.Mutex
	m  map[uint64]*epochState
}

type epochState struct {
	epoch        int64 // normalized epoch (ms); 0 = unknown
	lastRawStart int64 // last valid wire start_ts seen (ms); 0 if absent/invalid
	lastT        int64 // timestamp of the newest accepted sample (ms)
	lastV        float64
	stableCount  uint8 // consecutive points with an unchanged wire start
	lastSeenWall int64 // wall clock ms, for eviction
}

const (
	epochShardCount = 64
	// two consecutive points with the same start are enough to call the source
	// stable: churny sources change it on every point.
	epochStableThreshold = 2
	epochStableCap       = 200
	// state for series idle longer than this is evicted; on re-appearance the
	// series re-registers from the wire start (or epoch 0), which is safe.
	epochStateTTL   = 2 * time.Hour
	epochSweepEvery = 10 * time.Minute
)

func newEpochNormalizer() *epochNormalizer {
	n := &epochNormalizer{now: time.Now}
	for i := range n.shards {
		n.shards[i].m = make(map[uint64]*epochState)
	}
	return n
}

// normalize returns the epoch for a sample of series fp at time t (ms) with
// value v and wire start time rawStart (ms; 0 when the source did not send
// one). Out-of-order samples (t <= last accepted t) do not update state.
func (n *epochNormalizer) normalize(fp uint64, t int64, v float64, rawStart int64) int64 {
	rawValid := rawStart > 0 && rawStart <= t

	shard := &n.shards[fp%epochShardCount]
	shard.mu.Lock()
	defer shard.mu.Unlock()

	s, ok := shard.m[fp]
	nowWall := n.now().UnixMilli()
	if !ok {
		epoch := int64(0)
		if rawValid {
			epoch = rawStart
		}
		shard.m[fp] = &epochState{
			epoch:        epoch,
			lastRawStart: validOrZero(rawStart, rawValid),
			lastT:        t,
			lastV:        v,
			lastSeenWall: nowWall,
		}
		return epoch
	}

	s.lastSeenWall = nowWall

	if t <= s.lastT {
		// late or duplicate point: assign without disturbing tracking state.
		// A valid wire start is data; otherwise assume the current epoch.
		if rawValid {
			return rawStart
		}
		return s.epoch
	}

	wireStart := validOrZero(rawStart, rawValid)
	startChanged := wireStart != s.lastRawStart
	dropped := v < s.lastV

	switch {
	case !startChanged:
		if dropped {
			// value dropped inside a claimed epoch: the wire start does not
			// delimit monotone runs, distrust it (no synthesis — a made-up
			// timestamp would differ per replica and split the series)
			s.epoch = 0
			s.stableCount = 0
		} else {
			if s.stableCount < epochStableCap {
				s.stableCount++
			}
			// a previously distrusted series re-earns its epoch once the wire
			// start has held still across consecutive points
			if s.epoch == 0 && rawValid && s.stableCount >= 1 {
				s.epoch = rawStart
			}
		}
	case rawValid && dropped:
		// genuine reset with a fresh start time
		s.epoch = rawStart
		s.stableCount = 0
	case rawValid && s.stableCount >= epochStableThreshold:
		// start moved on a historically stable series: trust it even though the
		// value kept growing (restart + regrow past the previous value)
		s.epoch = rawStart
		s.stableCount = 0
	case rawValid:
		// start churns on consecutive points without a drop: distrust to 0 so
		// a monotone series is never shredded into per-point epochs
		s.epoch = 0
		s.stableCount = 0
	default:
		// start disappeared (or went invalid); keep the current epoch on value
		// continuity, distrust on a drop
		if dropped {
			s.epoch = 0
		}
		s.stableCount = 0
	}

	s.lastRawStart = wireStart
	s.lastT = t
	s.lastV = v
	return s.epoch
}

func validOrZero(rawStart int64, valid bool) int64 {
	if valid {
		return rawStart
	}
	return 0
}

// sweep drops state for series that have not been seen for epochStateTTL.
func (n *epochNormalizer) sweep() {
	cutoff := n.now().Add(-epochStateTTL).UnixMilli()
	for i := range n.shards {
		shard := &n.shards[i]
		shard.mu.Lock()
		for fp, s := range shard.m {
			if s.lastSeenWall < cutoff {
				delete(shard.m, fp)
			}
		}
		shard.mu.Unlock()
	}
}

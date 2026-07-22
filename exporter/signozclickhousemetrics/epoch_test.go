package signozclickhousemetrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEpochNormalizerStableStart(t *testing.T) {
	n := newEpochNormalizer()
	assert.Equal(t, int64(1000), n.normalize(1, 5000, 10, 1000))
	assert.Equal(t, int64(1000), n.normalize(1, 6000, 15, 1000))
	assert.Equal(t, int64(1000), n.normalize(1, 7000, 20, 1000))
}

func TestEpochNormalizerGenuineReset(t *testing.T) {
	n := newEpochNormalizer()
	assert.Equal(t, int64(1000), n.normalize(1, 5000, 100, 1000))
	assert.Equal(t, int64(1000), n.normalize(1, 6000, 110, 1000))
	// restart: new start time, value dropped
	assert.Equal(t, int64(6500), n.normalize(1, 7000, 3, 6500))
	assert.Equal(t, int64(6500), n.normalize(1, 8000, 9, 6500))
}

func TestEpochNormalizerRegrowPastPrevious(t *testing.T) {
	n := newEpochNormalizer()
	// three points with a stable start build trust
	n.normalize(1, 5000, 100, 1000)
	n.normalize(1, 6000, 110, 1000)
	n.normalize(1, 7000, 120, 1000)
	// restart whose counter regrew past 120 before the next export: the value
	// never drops, only the start time reveals the reset
	assert.Equal(t, int64(7500), n.normalize(1, 8000, 150, 7500))
}

func TestEpochNormalizerChurnGuard(t *testing.T) {
	n := newEpochNormalizer()
	// a spec-violating source advances start_time on every export while the
	// value grows monotonically: after the first sighting the series is
	// distrusted to 0 — never per-point epochs, and no replica-divergent pins
	assert.Equal(t, int64(1000), n.normalize(1, 5000, 10, 1000))
	assert.Equal(t, int64(0), n.normalize(1, 6000, 12, 5900))
	assert.Equal(t, int64(0), n.normalize(1, 7000, 15, 6900))
	assert.Equal(t, int64(0), n.normalize(1, 8000, 21, 7900))
}

func TestEpochNormalizerChurnThenStabilize(t *testing.T) {
	n := newEpochNormalizer()
	// a source that churns at startup and then holds still re-earns its epoch
	// once the wire start repeats — a wire value, so replicas converge on it
	n.normalize(1, 5000, 10, 1000)
	assert.Equal(t, int64(0), n.normalize(1, 6000, 12, 5900))
	assert.Equal(t, int64(0), n.normalize(1, 7000, 15, 6500))
	assert.Equal(t, int64(6500), n.normalize(1, 8000, 18, 6500))
	assert.Equal(t, int64(6500), n.normalize(1, 9000, 22, 6500))
}

func TestEpochNormalizerChurnWithDropsIsHonored(t *testing.T) {
	n := newEpochNormalizer()
	// start churns AND the value drops each time: every drop is a real reset
	assert.Equal(t, int64(1000), n.normalize(1, 5000, 10, 1000))
	assert.Equal(t, int64(5900), n.normalize(1, 6000, 2, 5900))
	assert.Equal(t, int64(6900), n.normalize(1, 7000, 1, 6900))
}

func TestEpochNormalizerAbsentStart(t *testing.T) {
	n := newEpochNormalizer()
	// no wire start, no epoch — ever. Synthesizing one from the drop time
	// would differ per collector replica and split the series into parallel
	// "writers"; these series keep the legacy read-path semantics instead.
	assert.Equal(t, int64(0), n.normalize(1, 5000, 10, 0))
	assert.Equal(t, int64(0), n.normalize(1, 6000, 15, 0))
	assert.Equal(t, int64(0), n.normalize(1, 7000, 2, 0))
	assert.Equal(t, int64(0), n.normalize(1, 8000, 8, 0))
	assert.Equal(t, int64(0), n.normalize(1, 9000, 1, 0))
}

func TestEpochNormalizerAbsentToPresentTransition(t *testing.T) {
	n := newEpochNormalizer()
	// source upgrade: starts sending start_time mid-stream with no reset; the
	// stable (absent) history makes the new start trustworthy
	n.normalize(1, 5000, 10, 0)
	n.normalize(1, 6000, 15, 0)
	n.normalize(1, 7000, 20, 0)
	assert.Equal(t, int64(1000), n.normalize(1, 8000, 25, 1000))
	assert.Equal(t, int64(1000), n.normalize(1, 9000, 30, 1000))
}

func TestEpochNormalizerInvalidStartIsAbsent(t *testing.T) {
	n := newEpochNormalizer()
	// start in the future of the sample is invalid
	assert.Equal(t, int64(0), n.normalize(1, 5000, 10, 9000))
	assert.Equal(t, int64(0), n.normalize(1, 6000, 12, 9000))
}

func TestEpochNormalizerStableStartWithDrop(t *testing.T) {
	n := newEpochNormalizer()
	// a value drop the wire didn't report (start unchanged): the start does
	// not delimit monotone runs, so the point is distrusted to 0; the epoch is
	// re-earned when the wire start holds still on subsequent points
	assert.Equal(t, int64(1000), n.normalize(1, 5000, 100, 1000))
	assert.Equal(t, int64(0), n.normalize(1, 6000, 5, 1000))
	assert.Equal(t, int64(1000), n.normalize(1, 7000, 9, 1000))
	assert.Equal(t, int64(1000), n.normalize(1, 8000, 14, 1000))
}

func TestEpochNormalizerOutOfOrder(t *testing.T) {
	n := newEpochNormalizer()
	n.normalize(1, 5000, 10, 1000)
	n.normalize(1, 6000, 15, 1000)
	// late point: gets an epoch, state untouched
	assert.Equal(t, int64(1000), n.normalize(1, 5500, 12, 1000))
	assert.Equal(t, int64(1000), n.normalize(1, 5500, 12, 0))
	// in-order flow continues unaffected
	assert.Equal(t, int64(1000), n.normalize(1, 7000, 20, 1000))
}

func TestEpochNormalizerSeriesIsolation(t *testing.T) {
	n := newEpochNormalizer()
	assert.Equal(t, int64(1000), n.normalize(1, 5000, 10, 1000))
	assert.Equal(t, int64(2000), n.normalize(2, 5000, 10, 2000))
	assert.Equal(t, int64(0), n.normalize(3, 5000, 10, 0))
}

func TestEpochNormalizerSweep(t *testing.T) {
	n := newEpochNormalizer()
	now := time.Unix(0, 0)
	n.now = func() time.Time { return now }
	n.normalize(1, 5000, 10, 1000)

	// not yet expired
	now = now.Add(epochStateTTL - time.Minute)
	n.sweep()
	shard := &n.shards[1%epochShardCount]
	shard.mu.Lock()
	_, ok := shard.m[1]
	shard.mu.Unlock()
	assert.True(t, ok)

	now = now.Add(2 * time.Minute)
	n.sweep()
	shard.mu.Lock()
	_, ok = shard.m[1]
	shard.mu.Unlock()
	assert.False(t, ok)
}

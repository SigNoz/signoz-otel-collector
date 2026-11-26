package fieldvalues

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
)

func bucketCapacity(bucketBytes uint64) int {
	return int(float64(bucketBytes/keyEntryBytes) * maxFill)
}

func newTestKeyCache(t *testing.T) *keyCache {
	t.Helper()
	c, err := newKeyCache(24*time.Hour, time.Hour, 0, 0.1, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	t.Cleanup(c.shutdown)
	return c
}

func TestKeyCacheCountsTheKeysOfTheWindow(t *testing.T) {
	c := newTestKeyCache(t)
	day := 20000 * dayMillis
	c.rotate(day)
	k1, k2 := mix64(1), mix64(2)
	noon := day + 12*hourMillis

	assert.False(t, c.known(k1, day, noon), "a new key is written")
	c.apply(day, noon, map[uint64]cacheClass{k1: classExact, k2: classReserve})
	assert.True(t, c.known(k1, day, noon+1000), "an applied key is known")
	assert.Equal(t, [2]int{1, 1}, c.used)

	c.limit = [2]int{1, 1}
	assert.False(t, c.room(classExact, 1), "a full class takes no new key, before the bucket drops keys")
	assert.False(t, c.room(classReserve, 1))
}

const (
	hourMillis   = uint64(time.Hour / time.Millisecond)
	minuteMillis = uint64(time.Minute / time.Millisecond)
)

// The window of this key starts 30 minutes after the window of the clock.
const keyAtHalfPast = 30 * minuteMillis

func TestKeyCacheWindowOfAKeyStartsAtItsOwnTime(t *testing.T) {
	c := newTestKeyCache(t)
	day, next := 20000*dayMillis, 20001*dayMillis
	k := keyAtHalfPast
	c.rotate(day)
	require.False(t, c.known(k, day, day+10*hourMillis))
	c.apply(day, day+10*hourMillis, map[uint64]cacheClass{k: classExact})

	c.rotate(next)
	assert.Equal(t, [2]int{0, 0}, c.used, "no key is carried into the new window")
	assert.True(t, c.known(k, next, next+29*minuteMillis), "before its own window starts, the key of the last window is known")
	assert.False(t, c.known(k, next, next+30*minuteMillis), "then it is written again")
	c.apply(next, next+30*minuteMillis, map[uint64]cacheClass{k: classExact})
	assert.True(t, c.known(k, next, next+31*minuteMillis))

	c.rotate(next + 2*dayMillis)
	assert.False(t, c.known(k, next+2*dayMillis, next+2*dayMillis+minuteMillis), "after a gap, no key is known")
}

// A set that is seen only in the first minutes of each day, such as a job at
// 00:05, must get its rows each day.
func TestKeyCacheKeySeenOnlyBeforeItsOwnTimeIsWrittenEachDay(t *testing.T) {
	c := newTestKeyCache(t)
	k := keyAtHalfPast
	for d := uint64(20000); d < 20003; d++ {
		day := d * dayMillis
		c.rotate(day)
		assert.False(t, c.known(k, day, day+5*minuteMillis), "day %d", d)
		c.apply(day, day+5*minuteMillis, map[uint64]cacheClass{k: classExact})
		assert.True(t, c.known(k, day, day+6*minuteMillis), "day %d", d)
		assert.Equal(t, [2]int{0, 0}, c.used, "the key counts for the last window, which holds it")
	}
}

func TestKeyCacheKeepsNewKeysInTheWindowWhenTheLastBucketIsFull(t *testing.T) {
	c := newTestKeyCache(t)
	day := 20000 * dayMillis
	k := keyAtHalfPast
	c.rotate(day)
	c.prevFree = 0
	require.False(t, c.known(k, day, day+5*minuteMillis))
	c.apply(day, day+5*minuteMillis, map[uint64]cacheClass{k: classExact})
	assert.True(t, c.known(k, day, day+6*minuteMillis))
	assert.True(t, c.known(k, day, day+31*minuteMillis), "the key is in the window of the clock, so it is not written again")
	assert.Equal(t, [2]int{1, 0}, c.used)
}

func TestKeyCacheSize(t *testing.T) {
	c, err := newKeyCache(24*time.Hour, 0, 256<<20, 0.1, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	t.Cleanup(c.shutdown)
	exact, reserve := c.capacity()
	assert.Equal(t, bucketCapacity(128<<20), exact+reserve, "two buckets of 128 MiB, 80% full")
	assert.InDelta(t, 0.1, float64(reserve)/float64(exact+reserve), 0.001)
}

func TestValueSet(t *testing.T) {
	tr := &tracker{budget: &budget{limit: 1 << 20}}
	st := &fieldState{}
	for i := uint64(0); i < 1000; i++ {
		assert.True(t, tr.add(st, mix64(i)))
	}
	assert.Equal(t, 1000, st.values.len())
	assert.Equal(t, 2048*valueSlotBytes, tr.bytes, "the table doubles at 75% full, here 16 bytes per value")
	for i := uint64(0); i < 1000; i += 2 {
		st.values.remove(mix64(i))
	}
	for i := uint64(0); i < 1000; i++ {
		assert.Equal(t, i%2 == 1, st.values.has(mix64(i)), "value %d", i)
	}
	assert.Equal(t, 500, st.values.len())
	var empty valueSet
	assert.False(t, empty.has(1))

	small := &tracker{budget: &budget{limit: minValueSlots * valueSlotBytes}}
	st2 := &fieldState{}
	for i := uint64(0); i < 6; i++ {
		assert.True(t, small.add(st2, mix64(i)))
	}
	assert.False(t, small.add(st2, mix64(100)), "the set cannot grow past the budget")
	small.spend(st2)
	assert.Zero(t, small.budget.used, "a spent field gives its memory back")
	assert.True(t, st2.spent)
}

func TestSketchCountsExactlyThenEstimates(t *testing.T) {
	var s sketch
	for i := 0; i < 20; i++ {
		s.add(mix64(uint64(i%10) + 1))
	}
	assert.Equal(t, 10.0, s.estimate())
	for i := 0; i < 5000; i++ {
		s.add(mix64(uint64(i) + 1))
	}
	assert.InEpsilon(t, 5000, s.estimate(), 0.4)
}

func TestKeyPoolSharesKeysAboveTheFloors(t *testing.T) {
	var p keyPool
	p.size.Store(20)
	a, b := newTestKeyCache(t), newTestKeyCache(t)
	a.share(&p, 10, 0)
	b.share(&p, 10, 0)
	assert.Equal(t, 30, a.limit[classExact], "the floor and the whole pool")

	a.used[classExact] = 25
	a.exactUsed.Store(25)
	assert.True(t, b.room(classExact, 10), "b keeps its floor")
	assert.True(t, b.room(classExact, 15), "and gets what a left in the pool")
	assert.False(t, b.room(classExact, 16))

	a.shutdown()
	assert.True(t, b.room(classExact, 30), "a cache that shuts down gives its keys back")
}

// A signal that stops gets no push, so its cache stays in the last window. Its
// draw must not hold the pool in the new window.
func TestKeyPoolIgnoresTheDrawOfAnEarlierWindow(t *testing.T) {
	var p keyPool
	p.size.Store(20)
	stopped, active := newTestKeyCache(t), newTestKeyCache(t)
	stopped.share(&p, 10, 0)
	active.share(&p, 10, 0)
	day := 20000 * dayMillis
	stopped.rotate(day)
	active.rotate(day)
	stopped.used[classExact] = 30
	stopped.exactUsed.Store(30)
	assert.False(t, active.room(classExact, 11), "in the same window, the pool is taken")

	active.rotate(day + dayMillis)
	assert.True(t, active.room(classExact, 30), "in the next window, the stopped signal holds nothing")
}

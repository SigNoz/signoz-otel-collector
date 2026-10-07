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

	write, _ := c.plan(k1, day, day+1000)
	assert.True(t, write, "a new key is written")
	c.apply(day, map[uint64]cacheClass{k1: classExact, k2: classReserve}, nil)
	write, _ = c.plan(k1, day, day+2000)
	assert.False(t, write, "an applied key is known")
	assert.Equal(t, [2]int{1, 1}, c.used)

	c.limit = [2]int{1, 1}
	assert.False(t, c.room(classExact, 1), "a full class takes no new key, before the bucket drops keys")
	assert.False(t, c.room(classReserve, 1))
}

func TestKeyCacheKeysWrittenAheadBecomeTheNextWindow(t *testing.T) {
	c := newTestKeyCache(t)
	day, next := 20000*dayMillis, 20001*dayMillis
	c.rotate(day)
	k := mix64(1)
	write, _ := c.plan(k, day, day+1000)
	require.True(t, write, "plan makes the bucket of the window; apply ignores keys of a bucket that does not exist")
	c.apply(day, map[uint64]cacheClass{k: classExact}, nil)

	_, ahead := c.plan(k, day, next-1)
	assert.True(t, ahead, "at the end of the pre-write window every known key is due")
	c.apply(day, nil, map[uint64]struct{}{k: {}})
	assert.Equal(t, 1, c.nextUsed)
	_, ahead = c.plan(k, day, next-1)
	assert.False(t, ahead, "a key written ahead is not due again")

	c.rotate(next)
	assert.Equal(t, [2]int{1, 0}, c.used, "after one step, the keys written ahead count for the window")
	write, _ = c.plan(k, next, next+1000)
	assert.False(t, write, "and they are known")

	c.rotate(next + 2*dayMillis)
	assert.Equal(t, [2]int{0, 0}, c.used, "after a gap, nothing is carried")
}

func TestKeyCacheAppliesKeysWrittenAheadAfterTheWindowChanged(t *testing.T) {
	c := newTestKeyCache(t)
	day, next := 20000*dayMillis, 20001*dayMillis
	c.rotate(day)
	k := mix64(1)
	write, _ := c.plan(k, day, day+1000)
	require.True(t, write, "plan makes the bucket of the window; apply ignores keys of a bucket that does not exist")
	c.apply(day, map[uint64]cacheClass{k: classExact}, nil)
	_, ahead := c.plan(k, day, next-1)
	require.True(t, ahead)

	c.rotate(next)
	c.apply(day, nil, map[uint64]struct{}{k: {}})
	assert.Equal(t, [2]int{1, 0}, c.used, "a batch of the last window adds its keys written ahead to the window")
	write, _ = c.plan(k, next, next+1000)
	assert.False(t, write)
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

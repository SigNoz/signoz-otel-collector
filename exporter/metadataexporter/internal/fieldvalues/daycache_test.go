package fieldvalues

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDayCacheSize(t *testing.T) {
	c := newDayCache(256<<20, 0.1)
	exact, reserve := c.capacity()
	assert.Equal(t, 33554432, len(c.slots), "256 MiB of 8-byte slots")
	assert.Equal(t, 26843545, exact+reserve, "80% of the slots")
	assert.InDelta(t, 0.1, float64(reserve)/float64(exact+reserve), 0.001)
}

func TestDayCacheKeepsTodaysKeysAndFreesOldDays(t *testing.T) {
	c := newDayCache(1<<20, 0.1)
	c.rotate(100)
	exact, _ := c.capacity()
	refused := 0
	for i := 0; i < exact; i++ {
		if !c.insert(mix64(uint64(i)+1), classExact) {
			refused++
		}
	}
	assert.LessOrEqual(t, refused, exact/1000, "two candidate buckets leave almost no key without a slot")
	for i := 0; c.room(classExact, 1); i++ {
		c.insert(mix64(uint64(i)+5_000_000), classExact)
	}
	assert.False(t, c.room(classExact, 1))
	assert.False(t, c.insert(mix64(uint64(exact)+1), classExact), "a full class takes no new key")
	assert.True(t, c.insert(mix64(uint64(exact)+1), classReserve), "the reserve still has room")
	kept := 0
	for i := 0; i < exact; i++ {
		if c.has(mix64(uint64(i) + 1)) {
			kept++
		}
	}
	assert.Equal(t, exact-refused, kept, "no key of today is evicted")

	c.rotate(101)
	assert.False(t, c.has(mix64(1)), "a key of yesterday does not count today")
	refused = 0
	for i := 0; i < exact; i++ {
		if !c.insert(mix64(uint64(i)+1_000_000), classExact) {
			refused++
		}
	}
	assert.LessOrEqual(t, refused, exact/1000, "the slots of yesterday are free")
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

package fieldvalues

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWindowCacheSize(t *testing.T) {
	c := newWindowCache(256<<20, 0.1)
	exact, reserve := c.capacity()
	assert.Equal(t, 33554432, len(c.slots), "256 MiB of 8-byte slots")
	assert.Equal(t, 26843545, exact+reserve, "80% of the slots")
	assert.InDelta(t, 0.1, float64(reserve)/float64(exact+reserve), 0.001)

	odd := newWindowCache(136<<20, 0.1)
	assert.Equal(t, (136<<20)/slotBytes, len(odd.slots), "a size that is not a power of two is used in full")
}

func TestWindowCacheKeepsKeysOfTheWindowAndFreesOldWindows(t *testing.T) {
	c := newWindowCache(1<<20, 0.1)
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
	assert.Equal(t, exact-refused, kept, "no key of the window is evicted")

	c.rotate(101)
	assert.False(t, c.has(mix64(1)), "a key of the last window does not count")
	refused = 0
	for i := 0; i < exact; i++ {
		if !c.insert(mix64(uint64(i)+1_000_000), classExact) {
			refused++
		}
	}
	assert.LessOrEqual(t, refused, exact/1000, "the slots of the last window are free")
}

func TestWindowCacheKeysWrittenAheadBecomeTheNextWindow(t *testing.T) {
	c := newWindowCache(1<<20, 0.1)
	c.rotate(100)
	current, ahead := mix64(1), mix64(2)
	assert.True(t, c.insert(current, classExact))
	assert.True(t, c.insertNext(ahead))
	assert.False(t, c.has(ahead), "a key written ahead is not a key of the window")
	assert.True(t, c.hasNext(ahead))
	assert.Equal(t, 1, c.nextUsed)

	c.rotate(101)
	assert.True(t, c.has(ahead), "after one step, the keys written ahead are the keys of the window")
	assert.False(t, c.has(current))
	assert.Equal(t, [2]int{1, 0}, c.used)
	assert.Zero(t, c.nextUsed)

	assert.True(t, c.insertNext(current))
	c.rotate(103)
	assert.False(t, c.has(current), "after a gap, keys written ahead for a skipped window do not count")
	assert.Equal(t, [2]int{0, 0}, c.used)
}

func TestWindowCacheClearsTagsOf256WindowsAgo(t *testing.T) {
	c := newWindowCache(1<<20, 0.1)
	c.rotate(100)
	k := mix64(7)
	assert.True(t, c.insert(k, classExact))
	for w := uint64(101); w < 100+256; w++ {
		c.rotate(w)
	}
	assert.False(t, c.hasNext(k), "the next window has the tag of window 100, so its old slot is cleared")
	c.rotate(100 + 256)
	assert.False(t, c.has(k), "a slot of 256 windows ago is not a key of the window")
}

func TestWindowCacheGivesSlotsWrittenAheadToTheWindow(t *testing.T) {
	c := newWindowCache(bucketSlots*slotBytes, 0.1)
	c.limit = [2]int{bucketSlots, 0}
	c.rotate(100)
	for i := 0; i < bucketSlots; i++ {
		assert.True(t, c.insertNext(mix64(uint64(i)+1)))
	}
	assert.Equal(t, bucketSlots, c.nextUsed)
	assert.True(t, c.insert(mix64(1000), classExact), "a key of the window takes the slot of a key written ahead")
	assert.Equal(t, bucketSlots-1, c.nextUsed)
	assert.False(t, c.insertNext(mix64(2000)), "the live keys of both windows fill the table")
}

func TestValueSet(t *testing.T) {
	tr := &tracker{budget: &budget{limit: 1 << 20}}
	st := &fieldState{}
	for i := uint64(0); i < 1000; i++ {
		assert.True(t, tr.add(st, mix64(i)))
	}
	assert.Equal(t, 1000, st.values.len())
	assert.Equal(t, 2048*slotBytes, tr.bytes, "the table doubles at 75% full, here 16 bytes per value")
	for i := uint64(0); i < 1000; i += 2 {
		st.values.remove(mix64(i))
	}
	for i := uint64(0); i < 1000; i++ {
		assert.Equal(t, i%2 == 1, st.values.has(mix64(i)), "value %d", i)
	}
	assert.Equal(t, 500, st.values.len())
	var empty valueSet
	assert.False(t, empty.has(1))

	small := &tracker{budget: &budget{limit: minValueSlots * slotBytes}}
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

package fieldvalues

// cacheClass is the part of the day cache that a key uses. Exact sets, their
// pairs and resource values use the exact part. Overflow sets use the reserve,
// so they still have room when the exact part is full.
type cacheClass uint8

const (
	classExact cacheClass = iota
	classReserve
)

const (
	bucketSlots = 8
	// maxFill keeps the table at most 80% full. With two candidate buckets per
	// key, almost no key of today is then pushed out early.
	maxFill = 0.8
	dayMask = uint64(0xff)
)

// dayCache remembers the keys written in the current UTC day, so that each set,
// pair and resource is written once per day. It is a fixed table of 8-byte
// keys in buckets of 8 slots, and a key can go into either of two buckets. The
// low 8 bits of a slot hold the day. A key of today is never evicted: when the
// part of a class is full, the caller degrades (coarse or overflow sets). Keys
// of an earlier day are free slots, so a new day needs no clear.
type dayCache struct {
	slots   []uint64
	buckets uint64
	day     uint64
	limit   [2]int
	used    [2]int
	// collisions counts keys that found no free slot although their class had
	// room. Such a key is written again at its next sighting.
	collisions int
}

func newDayCache(maxBytes uint64, reserveShare float64) *dayCache {
	buckets := uint64(1)
	for buckets*2*bucketSlots*8 <= maxBytes {
		buckets *= 2
	}
	capacity := int(float64(buckets*bucketSlots) * maxFill)
	reserve := int(float64(capacity) * reserveShare)
	return &dayCache{
		slots:   make([]uint64, buckets*bucketSlots),
		buckets: buckets,
		limit:   [2]int{capacity - reserve, reserve},
	}
}

func (c *dayCache) rotate(day uint64) {
	c.day = day & dayMask
	c.used = [2]int{}
}

func (c *dayCache) tagged(k uint64) uint64 {
	t := (k &^ dayMask) | c.day
	if t&^dayMask == 0 {
		t |= dayMask + 1
	}
	return t
}

func (c *dayCache) bucketsOf(k uint64) (uint64, uint64) {
	b1 := (k >> 8) & (c.buckets - 1)
	b2 := (mix64(k) >> 8) & (c.buckets - 1)
	return b1, b2
}

func (c *dayCache) has(k uint64) bool {
	t := c.tagged(k)
	b1, b2 := c.bucketsOf(k)
	return c.inBucket(b1, t) || c.inBucket(b2, t)
}

func (c *dayCache) inBucket(b, t uint64) bool {
	for _, s := range c.slots[b*bucketSlots : (b+1)*bucketSlots] {
		if s == t {
			return true
		}
	}
	return false
}

// room reports whether n more keys of the class fit today.
func (c *dayCache) room(class cacheClass, n int) bool {
	return c.used[class]+n <= c.limit[class]
}

// insert stores a key of today. It returns false when the class is full, or
// when both buckets are full of keys of today.
func (c *dayCache) insert(k uint64, class cacheClass) bool {
	if c.has(k) {
		return true
	}
	if !c.room(class, 1) {
		return false
	}
	t := c.tagged(k)
	b1, b2 := c.bucketsOf(k)
	first, second := b1, b2
	if c.todayIn(b2) < c.todayIn(b1) {
		first, second = b2, b1
	}
	for _, b := range []uint64{first, second} {
		bucket := c.slots[b*bucketSlots : (b+1)*bucketSlots]
		for i, s := range bucket {
			if s == 0 || s&dayMask != c.day {
				bucket[i] = t
				c.used[class]++
				return true
			}
		}
	}
	c.collisions++
	return false
}

func (c *dayCache) todayIn(b uint64) int {
	n := 0
	for _, s := range c.slots[b*bucketSlots : (b+1)*bucketSlots] {
		if s != 0 && s&dayMask == c.day {
			n++
		}
	}
	return n
}

// capacity is the number of keys of today that each class can hold.
func (c *dayCache) capacity() (exact, reserve int) {
	return c.limit[classExact], c.limit[classReserve]
}

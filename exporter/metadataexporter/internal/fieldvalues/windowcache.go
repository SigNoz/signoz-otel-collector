package fieldvalues

import "math/bits"

// cacheClass is the part of the window cache that a key uses. Exact sets,
// their pairs and resource values use the exact part. Overflow sets use the
// reserve, so they still have room when the exact part is full.
type cacheClass uint8

const (
	classExact cacheClass = iota
	classReserve
)

const (
	bucketSlots = 8
	// maxFill keeps the table at most 80% full. With two candidate buckets per
	// key, almost no key of the window is then pushed out early.
	maxFill   = 0.8
	tagMask   = uint64(0xff)
	slotBytes = 8
)

// windowCache remembers the keys written in the current window, so that each
// set, pair and resource is written once per window. It is a fixed table of
// 8-byte keys in buckets of 8 slots, and a key can go into either of two
// buckets. The low 8 bits of a slot are a tag of its window.
//
// Two windows are live: the current one, and the next one, which holds the
// keys written ahead in the last part of the current window. A key of the
// current window is never evicted: when the part of a class is full, the
// caller degrades (coarse or overflow sets). A key written ahead gives its
// slot to a key of the current window when both buckets are full. Slots of
// other windows are free, so a new window needs no clear.
type windowCache struct {
	slots   []uint64
	buckets uint64
	window  uint64
	cur     uint64
	next    uint64
	limit   [2]int
	used    [2]int
	// nextUsed counts the keys written ahead for the next window.
	nextUsed int
	// tags counts the used slots of each tag, so that a rotation scans the
	// table only when a stale tag can be in it.
	tags       [tagMask + 1]int
	collisions int
}

func newWindowCache(maxBytes uint64, reserveShare float64) *windowCache {
	buckets := max(maxBytes/(bucketSlots*slotBytes), 1)
	capacity := int(float64(buckets*bucketSlots) * maxFill)
	reserve := int(float64(capacity) * reserveShare)
	return &windowCache{
		slots:   make([]uint64, buckets*bucketSlots),
		buckets: buckets,
		limit:   [2]int{capacity - reserve, reserve},
	}
}

// rotate makes window the current window. After a step of one window, the
// slots with the new current tag are the keys written ahead, and they stay.
// Any other slot with the current or next tag is from 256 windows ago, and is
// cleared.
func (c *windowCache) rotate(window uint64) {
	step := window == c.window+1
	carried := 0
	if step {
		carried = c.nextUsed
	}
	c.window, c.cur, c.next = window, window&tagMask, (window+1)&tagMask
	clearCur := !step && c.tags[c.cur] > 0
	if clearCur || c.tags[c.next] > 0 {
		for i, s := range c.slots {
			if s == 0 {
				continue
			}
			if t := s & tagMask; t == c.next || (clearCur && t == c.cur) {
				c.set(i, 0)
			}
		}
	}
	c.used = [2]int{carried, 0}
	c.nextUsed = 0
}

func (c *windowCache) set(i int, s uint64) {
	if old := c.slots[i]; old != 0 {
		c.tags[old&tagMask]--
	}
	if s != 0 {
		c.tags[s&tagMask]++
	}
	c.slots[i] = s
}

func (c *windowCache) tagged(k, tag uint64) uint64 {
	t := (k &^ tagMask) | tag
	if t&^tagMask == 0 {
		t |= tagMask + 1
	}
	return t
}

// bucketsOf maps a key to two buckets with the high bits of a product, so the
// number of buckets needs not be a power of two.
func (c *windowCache) bucketsOf(k uint64) (uint64, uint64) {
	b1, _ := bits.Mul64(k, c.buckets)
	b2, _ := bits.Mul64(mix64(k), c.buckets)
	return b1, b2
}

func (c *windowCache) has(k uint64) bool {
	return c.hasTag(k, c.cur)
}

func (c *windowCache) hasNext(k uint64) bool {
	return c.hasTag(k, c.next)
}

func (c *windowCache) hasTag(k, tag uint64) bool {
	t := c.tagged(k, tag)
	b1, b2 := c.bucketsOf(k)
	return c.find(b1, t) || c.find(b2, t)
}

func (c *windowCache) find(b, t uint64) bool {
	start := b * bucketSlots
	for _, s := range c.slots[start : start+bucketSlots] {
		if s == t {
			return true
		}
	}
	return false
}

// room reports whether n more keys of the class fit in the current window.
func (c *windowCache) room(class cacheClass, n int) bool {
	return c.used[class]+n <= c.limit[class]
}

// roomNext reports whether n more keys can be written ahead. Keys written
// ahead use the exact part of the next window, and the live keys of both
// windows stay within the capacity of the table.
func (c *windowCache) roomNext(n int) bool {
	return c.nextUsed+n <= c.limit[classExact] &&
		c.used[classExact]+c.used[classReserve]+c.nextUsed+n <= c.limit[classExact]+c.limit[classReserve]
}

// insert stores a key of the current window. It returns false when the class
// is full, or when both buckets are full of keys of the current window.
func (c *windowCache) insert(k uint64, class cacheClass) bool {
	if c.has(k) {
		return true
	}
	if !c.room(class, 1) {
		return false
	}
	t := c.tagged(k, c.cur)
	b1, b2 := c.bucketsOf(k)
	if i := c.freeSlot(b1, b2); i >= 0 {
		c.set(i, t)
		c.used[class]++
		return true
	}
	for _, b := range [2]uint64{b1, b2} {
		start := int(b * bucketSlots)
		for i, s := range c.slots[start : start+bucketSlots] {
			if s != 0 && s&tagMask == c.next {
				c.set(start+i, t)
				c.nextUsed--
				c.used[class]++
				return true
			}
		}
	}
	c.collisions++
	return false
}

// insertNext stores a key written ahead for the next window.
func (c *windowCache) insertNext(k uint64) bool {
	if c.hasNext(k) {
		return true
	}
	if !c.roomNext(1) {
		return false
	}
	b1, b2 := c.bucketsOf(k)
	if i := c.freeSlot(b1, b2); i >= 0 {
		c.set(i, c.tagged(k, c.next))
		c.nextUsed++
		return true
	}
	c.collisions++
	return false
}

// freeSlot returns a slot of no live window, in the less used of the two
// buckets first, or -1.
func (c *windowCache) freeSlot(b1, b2 uint64) int {
	first, second := b1, b2
	if c.live(b2) < c.live(b1) {
		first, second = b2, b1
	}
	for _, b := range [2]uint64{first, second} {
		start := int(b * bucketSlots)
		for i, s := range c.slots[start : start+bucketSlots] {
			if t := s & tagMask; s == 0 || (t != c.cur && t != c.next) {
				return start + i
			}
		}
	}
	return -1
}

func (c *windowCache) live(b uint64) int {
	n := 0
	start := b * bucketSlots
	for _, s := range c.slots[start : start+bucketSlots] {
		if t := s & tagMask; s != 0 && (t == c.cur || t == c.next) {
			n++
		}
	}
	return n
}

// capacity is the number of keys of a window that each class can hold.
func (c *windowCache) capacity() (exact, reserve int) {
	return c.limit[classExact], c.limit[classReserve]
}

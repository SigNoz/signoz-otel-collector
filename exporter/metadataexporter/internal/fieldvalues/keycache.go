package fieldvalues

import (
	"encoding/binary"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/otel/attribute"

	"github.com/SigNoz/signoz-otel-collector/pkg/timebucketedset"
)

// cacheClass is the part of the key cache that a key uses. Exact sets, their
// pairs and resource values use the exact part. Overflow sets use the
// reserve, so they still have room when the exact part is full.
type cacheClass uint8

const (
	classExact cacheClass = iota
	classReserve
)

const (
	// cacheBuckets holds the current window and the next one, for the keys
	// written ahead.
	cacheBuckets = 2
	// minBucketBytes is the smallest bucket of pkg/timebucketedset.
	minBucketBytes = 32 << 20
	// keyEntryBytes is the memory of one key in a bucket: 8 bytes and a
	// header of 4.
	keyEntryBytes = 12
	// maxFill keeps the keys of a window below 80% of the bucket, so the
	// bucket does not drop its oldest keys.
	maxFill = 0.8
)

// keyCache remembers the keys written in the current window, so that each
// set, pair and resource is written once per window. pkg/timebucketedset
// keeps the keys, one bucket per window, and tells which keys to write ahead
// for the next window. keyCache counts the keys of the window, so that the
// writer degrades to coarse and overflow sets before a bucket is full and
// drops its oldest keys.
type keyCache struct {
	set    *timebucketedset.Set
	width  uint64
	window uint64
	limit  [2]int
	used   [2]int
	// nextUsed counts the keys written ahead for the next window.
	nextUsed int
	// pool, floor and exactUsed share keys with the caches of other writers:
	// the exact part may pass its floor while the pool has room. exactUsed is
	// used[classExact], and exactWindow its window, for the other caches of
	// the pool to read.
	pool        *keyPool
	floor       int
	exactUsed   atomic.Int64
	exactWindow atomic.Uint64
}

// keyPool lets the key caches of the writers with no max_bytes share their
// keys. Each cache has a floor of its own. Above the floors, the caches draw
// on one pool as they need keys in the window, so the signal with the most
// sets gets the most keys, on each tenant.
type keyPool struct {
	size   atomic.Int64
	mu     sync.Mutex
	caches atomic.Pointer[[]*keyCache]
}

func (p *keyPool) add(c *keyCache) {
	p.mu.Lock()
	defer p.mu.Unlock()
	var caches []*keyCache
	if old := p.caches.Load(); old != nil {
		caches = slices.Clone(*old)
	}
	caches = append(caches, c)
	p.caches.Store(&caches)
}

func (p *keyPool) remove(c *keyCache) {
	p.mu.Lock()
	defer p.mu.Unlock()
	old := p.caches.Load()
	if old == nil {
		return
	}
	caches := slices.DeleteFunc(slices.Clone(*old), func(o *keyCache) bool { return o == c })
	p.caches.Store(&caches)
}

// allows reports whether a cache may hold want keys in its exact part: within
// its floor, or above it while the pool has room. A cache rotates only at a
// push, so the draw of a cache that is still in an earlier window (a signal
// that stopped) does not count.
func (p *keyPool) allows(c *keyCache, want int) bool {
	drawn := want - c.floor
	if drawn <= 0 {
		return true
	}
	if caches := p.caches.Load(); caches != nil {
		for _, o := range *caches {
			if o != c && o.exactWindow.Load() == c.window {
				drawn += max(0, int(o.exactUsed.Load())-o.floor)
			}
		}
	}
	return int64(drawn) <= p.size.Load()
}

// windowKeys is the number of keys of a window that maxBytes of key cache
// holds.
func windowKeys(maxBytes uint64) int {
	return int(float64(maxBytes/cacheBuckets/keyEntryBytes) * maxFill)
}

func newKeyCache(width, preWrite time.Duration, maxBytes uint64, reserveShare float64, settings component.TelemetrySettings, identifiers ...attribute.KeyValue) (*keyCache, error) {
	bucketBytes := max(maxBytes/cacheBuckets, minBucketBytes)
	set, err := timebucketedset.New(width, timebucketedset.Config{
		MaxBuckets:     cacheBuckets,
		MaxBucketSize:  int(bucketBytes),
		PreWriteWindow: preWrite,
	}, settings, identifiers...)
	if err != nil {
		return nil, err
	}
	capacity := int(float64(bucketBytes/keyEntryBytes) * maxFill)
	reserve := int(float64(capacity) * reserveShare)
	return &keyCache{
		set:   set,
		width: uint64(width.Milliseconds()),
		limit: [2]int{capacity - reserve, reserve},
	}, nil
}

func keyID(k uint64) [8]byte {
	var id [8]byte
	binary.LittleEndian.PutUint64(id[:], k)
	return id
}

// plan reports whether a key must be written in the window that starts at
// window, and whether it is due to be written ahead for the next window.
func (c *keyCache) plan(k, window, now uint64) (write, ahead bool) {
	id := keyID(k)
	return c.set.Plan(id[:], int64(window), int64(now))
}

// rotate makes the window that starts at window the current window. After a
// step of one window, the keys written ahead are its keys.
func (c *keyCache) rotate(window uint64) {
	carried := 0
	if window == c.window+c.width {
		carried = c.nextUsed
	}
	c.window = window
	c.used = [2]int{carried, 0}
	c.nextUsed = 0
	c.exactUsed.Store(int64(carried))
	c.exactWindow.Store(window)
}

// share makes the cache draw on a pool: its own floor of keys, of which the
// reserve is a part, and the exact part may grow by the size of the pool.
func (c *keyCache) share(p *keyPool, floor int, reserveShare float64) {
	reserve := int(float64(floor) * reserveShare)
	c.pool = p
	c.floor = floor - reserve
	c.limit = [2]int{floor - reserve + int(p.size.Load()), reserve}
	p.add(c)
}

// room reports whether n more keys of the class fit in the current window.
func (c *keyCache) room(class cacheClass, n int) bool {
	if c.used[class]+n > c.limit[class] {
		return false
	}
	return class != classExact || c.pool == nil || c.pool.allows(c, c.used[classExact]+n)
}

// roomNext reports whether n more keys can be written ahead for the next
// window.
func (c *keyCache) roomNext(n int) bool {
	if c.nextUsed+n > c.limit[classExact] {
		return false
	}
	return c.pool == nil || c.pool.allows(c, c.nextUsed+n)
}

// apply stores the keys of a written batch of the window that starts at
// window: keys for that window, and keys written ahead for the one after.
// Keys of a window that is no longer live are ignored.
func (c *keyCache) apply(window uint64, keys map[uint64]cacheClass, ahead map[uint64]struct{}) {
	id := make([]byte, 8)
	c.set.Apply(func(yield func([]byte, int64) bool) {
		for k := range keys {
			binary.LittleEndian.PutUint64(id, k)
			if !yield(id, int64(window)) {
				return
			}
		}
		for k := range ahead {
			binary.LittleEndian.PutUint64(id, k)
			if !yield(id, int64(window+c.width)) {
				return
			}
		}
	})
	switch window {
	case c.window:
		for _, class := range keys {
			c.used[class]++
		}
		c.nextUsed += len(ahead)
	case c.window - c.width:
		c.used[classExact] += len(ahead)
	}
	c.exactUsed.Store(int64(c.used[classExact]))
}

// capacity is the number of keys of a window that each class can hold.
func (c *keyCache) capacity() (exact, reserve int) {
	return c.limit[classExact], c.limit[classReserve]
}

func (c *keyCache) shutdown() {
	if c.pool != nil {
		c.pool.remove(c)
	}
	c.set.Shutdown()
}

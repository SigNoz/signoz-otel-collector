// Package bucketset tracks, per time bucket, the identities whose registration
// row has already been written to ClickHouse, so an exporter writes one row per
// identity per bucket instead of one per batch.
//
// A Set may forget an identity, which costs a duplicate row that
// ReplacingMergeTree collapses. It never reports an identity as registered
// before Commit.
package bucketsetcache

import (
	"encoding/binary"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/VictoriaMetrics/fastcache"
	"github.com/cespare/xxhash/v2"
)

type Set struct {
	cfg      Config
	width    int64
	preWrite int64

	// mu is read-held for the whole of Plan and Commit so that eviction, which
	// resets and reuses a generation, cannot interleave with a lookup or mark on it.
	mu           sync.RWMutex
	gens         map[int64]*fastcache.Cache
	spare        []*fastcache.Cache
	evictions    atomic.Uint64
	droppedMarks atomic.Uint64
}

func New(width time.Duration, cfg Config) (*Set, error) {
	cfg = cfg.withDefaults()
	if err := cfg.Validate(width); err != nil {
		return nil, err
	}
	return &Set{
		cfg:      cfg,
		width:    width.Milliseconds(),
		preWrite: cfg.PreWriteWindow.Milliseconds(),
		gens:     make(map[int64]*fastcache.Cache, cfg.MaxBuckets),
	}, nil
}

func (s *Set) BucketStart(unixMilli int64) int64 {
	return unixMilli / s.width * s.width
}

// Plan reports the registration rows an observation of id in the bucket
// starting at bucketStart requires: cur is the row for bucketStart itself, next
// the pre-written row for the following bucket. It never marks; Commit the rows
// once they have been sent.
func (s *Set) Plan(id []byte, bucketStart, nowMilli int64) (cur, next bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	g := s.generation(bucketStart, nowMilli)
	if g == nil {
		return true, false
	}
	cur = !g.Has(id)
	if cur || s.preWrite == 0 || !s.inPreWriteSlot(id, bucketStart, nowMilli) {
		return cur, false
	}
	ng := s.generation(bucketStart+s.width, nowMilli)
	return false, ng != nil && !ng.Has(id)
}

// inPreWriteSlot staggers pre-writes across the window: identity i becomes
// eligible at bucketEnd-window+(hash(i) mod window).
func (s *Set) inPreWriteSlot(id []byte, bucketStart, nowMilli int64) bool {
	into := nowMilli - bucketStart
	start := s.width - s.preWrite
	if into < start || into >= s.width {
		return false
	}
	return into-start >= int64(xxhash.Sum64(id)%uint64(s.preWrite))
}

// generation returns the live set for bucketStart, creating it unless the
// bucket is more than one width in the future or older than every live bucket
// while the ring is full. The caller holds s.mu for reading; creation upgrades
// to a write lock and downgrades again, so the caller's view of s.gens may have
// changed on return.
func (s *Set) generation(bucketStart, nowMilli int64) *fastcache.Cache {
	if bucketStart > s.BucketStart(nowMilli)+s.width {
		return nil
	}
	if g, ok := s.gens[bucketStart]; ok {
		return g
	}
	s.mu.RUnlock()
	s.mu.Lock()
	s.create(bucketStart)
	s.mu.Unlock()
	s.mu.RLock()
	return s.gens[bucketStart]
}

func (s *Set) create(bucketStart int64) {
	if _, ok := s.gens[bucketStart]; ok {
		return
	}
	if len(s.gens) >= s.cfg.MaxBuckets {
		oldest := bucketStart
		for b := range s.gens {
			if b < oldest {
				oldest = b
			}
		}
		if oldest == bucketStart {
			return
		}
		g := s.gens[oldest]
		delete(s.gens, oldest)
		g.Reset()
		s.spare = append(s.spare, g)
		s.evictions.Add(1)
	}
	var g *fastcache.Cache
	if n := len(s.spare); n > 0 {
		g = s.spare[n-1]
		s.spare = s.spare[:n-1]
	} else {
		g = fastcache.New(s.cfg.MaxBytes)
	}
	s.gens[bucketStart] = g
}

// Pending collects the rows appended to one batch so they can be marked
// together after the batch is sent.
type Pending struct {
	ids     [][]byte
	buckets []int64
}

// Add copies id, since callers reuse their key buffers.
func (p *Pending) Add(id []byte, bucketStart int64) {
	p.ids = append(p.ids, append([]byte(nil), id...))
	p.buckets = append(p.buckets, bucketStart)
}

func (p *Pending) Len() int {
	return len(p.ids)
}

// Commit marks every pending row as registered and empties p. Rows whose
// bucket is no longer live are dropped and get re-planned on the next batch.
func (s *Set) Commit(p *Pending) {
	s.mu.RLock()
	for i, id := range p.ids {
		g, ok := s.gens[p.buckets[i]]
		if !ok {
			s.droppedMarks.Add(1)
			continue
		}
		g.Set(id, nil)
	}
	s.mu.RUnlock()
	clear(p.ids)
	p.ids = p.ids[:0]
	p.buckets = p.buckets[:0]
}

type Stats struct {
	Buckets   int
	Entries   uint64
	Bytes     uint64
	Evictions uint64
	// DroppedMarks counts Commit entries whose bucket had already been evicted.
	DroppedMarks uint64
	// Collisions counts fastcache index collisions in live buckets; each one
	// turned into a duplicate row.
	Collisions uint64
}

func (s *Set) Stats() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var fs fastcache.Stats
	for _, g := range s.gens {
		g.UpdateStats(&fs)
	}
	return Stats{
		Buckets:      len(s.gens),
		Entries:      fs.EntriesCount,
		Bytes:        fs.BytesSize,
		Evictions:    s.evictions.Load(),
		DroppedMarks: s.droppedMarks.Load(),
		Collisions:   fs.Collisions,
	}
}

// SeriesID packs a series fingerprint and its reduced flag into dst, so the raw
// and reduced rows of one fingerprint are distinct identities.
func SeriesID(dst *[9]byte, fingerprint uint64, reduced bool) []byte {
	binary.LittleEndian.PutUint64(dst[:8], fingerprint)
	dst[8] = 0
	if reduced {
		dst[8] = 1
	}
	return dst[:]
}

// StringKey returns the bytes of s without copying. Plan and Pending.Add only
// read their key argument, so the alias is safe there and nowhere else.
func StringKey(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

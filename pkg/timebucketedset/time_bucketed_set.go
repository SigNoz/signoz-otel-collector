package timebucketedset

import (
	"fmt"
	"iter"
	"sync"
	"time"

	"github.com/VictoriaMetrics/fastcache"
	"github.com/cespare/xxhash/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/otel/attribute"
)

type Set struct {
	// The input config of the bucket set.
	config Config

	// The map of all buckets keyed by bucket start being held by this bucketset.
	buckets map[int64]*fastcache.Cache

	// Buckets which are unused are kept here to prevent re-allocating buckets.
	spareBuckets []*fastcache.Cache

	// The duration in milliseconds of each bucket in the bucketset.
	width int64

	// The duration in which items are written to the next bucket.
	preWriteWindow int64

	// Read Write mutex for internal bucket set operations.
	mtx sync.RWMutex

	telemetry *telemetry
}

// New reports metrics through settings, each carrying identifiers. Pass the
// owning component's kind and ID, e.g. attribute.String("exporter",
// set.ID.String()), plus anything that tells this set apart from others the
// component owns.
func New(width time.Duration, config Config, settings component.TelemetrySettings, identifiers ...attribute.KeyValue) (*Set, error) {
	config = config.WithDefaults()
	if err := config.Validate(width); err != nil {
		return nil, err
	}

	telemetry, err := newTelemetry(settings, identifiers)
	if err != nil {
		return nil, fmt.Errorf("time_bucketed_set::telemetry: %w", err)
	}

	bs := &Set{
		config:         config,
		buckets:        make(map[int64]*fastcache.Cache, config.MaxBuckets),
		width:          width.Milliseconds(),
		preWriteWindow: config.PreWriteWindow.Milliseconds(),
		telemetry:      telemetry,
	}

	if err := telemetry.register(bs); err != nil {
		telemetry.builder.Shutdown()
		return nil, fmt.Errorf("time_bucketed_set::telemetry: %w", err)
	}

	return bs, nil
}

// Shutdown stops reporting metrics. The set stays usable.
func (bs *Set) Shutdown() {
	bs.telemetry.builder.Shutdown()
}

func (bs *Set) BucketStart(unixMilliseconds int64) int64 {
	return unixMilliseconds / bs.width * bs.width
}

func (bs *Set) Plan(id []byte, bucketStartUnixMilliseconds int64, unixMilliseconds int64) (bool, bool) {
	bs.mtx.RLock()
	defer bs.mtx.RUnlock()

	currentBucket := bs.getOrCreateBucket(bucketStartUnixMilliseconds, unixMilliseconds)
	if currentBucket == nil {
		bs.telemetry.noBucket.Add(1)
		return true, false
	}

	// If the current bucket does not have the id or it's not in the pre-write window, next will always be false.
	current := !currentBucket.Has(id)
	if current {
		bs.telemetry.miss.Add(1)
		return true, false
	}
	if bs.preWriteWindow == 0 || !bs.isInPreWriteWindow(id, bucketStartUnixMilliseconds, unixMilliseconds) {
		bs.telemetry.hit.Add(1)
		return false, false
	}

	nextBucket := bs.getOrCreateBucket(bucketStartUnixMilliseconds+bs.width, unixMilliseconds)
	next := nextBucket != nil && !nextBucket.Has(id)
	if next {
		bs.telemetry.preWrite.Add(1)
	} else {
		bs.telemetry.hit.Add(1)
	}
	return false, next
}

// Apply marks every yielded (id, bucket start) as registered. Ids are only
// read during the iteration, so the producer may reuse one key buffer across
// yields. Rows for a bucket that is no longer live are ignored; the next Plan
// for them is true again.
func (bs *Set) Apply(rows iter.Seq2[[]byte, int64]) {
	bs.mtx.RLock()
	defer bs.mtx.RUnlock()

	var applied, ignored int64
	for id, bucketStartUnixMilliseconds := range rows {
		bucket, ok := bs.buckets[bucketStartUnixMilliseconds]
		if !ok {
			ignored++
			continue
		}
		bucket.Set(id, nil)
		applied++
	}
	bs.telemetry.applied.Add(applied)
	bs.telemetry.ignored.Add(ignored)
}

func (bs *Set) getOrCreateBucket(bucketStartUnixMilliseconds int64, unixMilliseconds int64) *fastcache.Cache {
	// If bucket start is greater than one full width into the future
	if bucketStartUnixMilliseconds > bs.BucketStart(unixMilliseconds)+bs.width {
		return nil
	}

	if bucket, ok := bs.buckets[bucketStartUnixMilliseconds]; ok {
		return bucket
	}

	bs.mtx.RUnlock()
	bs.mtx.Lock()
	bs.createBucket(bucketStartUnixMilliseconds)
	bs.mtx.Unlock()
	bs.mtx.RLock()

	return bs.buckets[bucketStartUnixMilliseconds]
}

func (bs *Set) createBucket(bucketStartUnixMilliseconds int64) {
	// If a bucket exists for this bucket start, do nothing.
	if _, ok := bs.buckets[bucketStartUnixMilliseconds]; ok {
		return
	}

	// If the number of current buckets has crossed the max buckets limit, drop the oldest bucket
	if len(bs.buckets) >= bs.config.MaxBuckets {
		oldest := bucketStartUnixMilliseconds
		for b := range bs.buckets {
			if b < oldest {
				oldest = b
			}
		}

		// If the oldest bucket is the input bucket, do nothing.
		if oldest == bucketStartUnixMilliseconds {
			return
		}

		// delete from the buckets map
		bucket := bs.buckets[oldest]
		delete(bs.buckets, oldest)
		bs.telemetry.evictions.Add(1)

		// reset and add to spare buckets
		bucket.Reset()
		bs.spareBuckets = append(bs.spareBuckets, bucket)
	}

	var bucket *fastcache.Cache
	if numSpareBuckets := len(bs.spareBuckets); numSpareBuckets > 0 {
		// There are available spare buckets, take the last spare bucket
		bucket = bs.spareBuckets[numSpareBuckets-1]
		bs.spareBuckets = bs.spareBuckets[:numSpareBuckets-1]
	} else {
		// allocate a new bucket
		bucket = fastcache.New(bs.config.MaxBucketSize)
	}

	bs.buckets[bucketStartUnixMilliseconds] = bucket
}

func (bs *Set) isInPreWriteWindow(id []byte, bucketStartUnixMilliseconds int64, unixMilliseconds int64) bool {
	current := unixMilliseconds - bucketStartUnixMilliseconds
	preWriteStart := bs.width - bs.preWriteWindow

	if current < preWriteStart || current >= bs.width {
		return false
	}

	// item id becomes eligible for pre write at bucketEnd - preWriteWindow + hash(id) mod preWriteWindow so eligibility is staggered
	return current >= preWriteStart+int64(xxhash.Sum64(id)%uint64(bs.preWriteWindow))
}

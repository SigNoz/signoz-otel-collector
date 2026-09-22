package timebucketedset

import (
	"sync"
	"time"

	"github.com/VictoriaMetrics/fastcache"
	"github.com/cespare/xxhash/v2"
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
}

func New(width time.Duration, config Config) (*Set, error) {
	config = config.WithDefaults()
	if err := config.Validate(width); err != nil {
		return nil, err
	}

	return &Set{
		config:         config,
		buckets:        make(map[int64]*fastcache.Cache, config.MaxBuckets),
		width:          width.Milliseconds(),
		preWriteWindow: config.PreWriteWindow.Milliseconds(),
	}, nil

}

func (bs *Set) BucketStart(unixMilliseconds int64) int64 {
	return unixMilliseconds / bs.width * bs.width
}

func (bs *Set) Plan(id []byte, bucketStartUnixMilliseconds int64, unixMilliseconds int64) (bool, bool) {
	bs.mtx.RLock()
	defer bs.mtx.RUnlock()

	currentBucket := bs.getOrCreateBucket(bucketStartUnixMilliseconds, unixMilliseconds)
	if currentBucket == nil {
		return true, false
	}

	// If the current bucket does not have the id or it's not in the pre-write window, next will always be false.
	current := !currentBucket.Has(id)
	if current || bs.preWriteWindow == 0 || !bs.isInPreWriteWindow(id, bucketStartUnixMilliseconds, unixMilliseconds) {
		return current, false
	}

	nextBucket := bs.getOrCreateBucket(bucketStartUnixMilliseconds+bs.width, unixMilliseconds)
	return false, nextBucket != nil && !nextBucket.Has(id)
}

func (bs *Set) Apply(items *Items) {
	bs.mtx.RLock()
	for i, id := range items.ids {
		bucket, ok := bs.buckets[items.bucketKeys[i]]
		if !ok {
			continue
		}

		bucket.Set(id, nil)
	}
	bs.mtx.RUnlock()
	items.Reset()
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

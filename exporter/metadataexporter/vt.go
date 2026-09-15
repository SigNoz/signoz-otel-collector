package metadataexporter

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/jellydator/ttlcache/v3"
)

// trackedValues is the set of distinct values seen for one key. Once the set
// grows past the configured limit the values are released and only the
// over-limit flag is kept.
type trackedValues struct {
	mu        sync.Mutex
	values    map[string]struct{}
	overLimit bool
	// touched is when the entry's TTL was last extended.
	touched time.Time
}

// trackerTouchInterval is how often an entry that keeps being accessed has
// its TTL extended; extending it on every access re-arms the cache's
// expiration timer each time.
const trackerTouchInterval = time.Minute

// ValueTracker records the distinct values seen per key within a TTL window
// and reports which keys have exceeded the distinct-value limit. An entry's TTL
// is extended while it keeps being accessed, so a key stays over the limit for
// as long as it keeps arriving and is re-admitted only after it has been
// absent for a full TTL.
type ValueTracker struct {
	ttl             *ttlcache.Cache[string, *trackedValues]
	maxValuesPerKey int
}

// NewValueTracker returns a tracker that flags a key once more than
// maxValuesPerKey distinct values are seen for it. A non-positive
// maxValuesPerKey disables the distinct-value limit; keys can then only be
// flagged through MarkOverLimit.
func NewValueTracker(
	maxKeys int,
	maxValuesPerKey int,
	ttl time.Duration,
) *ValueTracker {
	cache := ttlcache.New(
		ttlcache.WithTTL[string, *trackedValues](ttl),
		ttlcache.WithCapacity[string, *trackedValues](uint64(maxKeys)),
		ttlcache.WithDisableTouchOnHit[string, *trackedValues](),
	)

	go cache.Start()

	return &ValueTracker{
		ttl:             cache,
		maxValuesPerKey: maxValuesPerKey,
	}
}

func (vt *ValueTracker) entry(key string) *trackedValues {
	item, _ := vt.ttl.GetOrSetFunc(key, newTrackedValues)
	tv := item.Value()
	vt.touch(key, tv)
	return tv
}

func newTrackedValues() *trackedValues {
	return &trackedValues{values: make(map[string]struct{}), touched: time.Now()}
}

// touch extends the entry's TTL once per trackerTouchInterval.
func (vt *ValueTracker) touch(key string, tv *trackedValues) {
	now := time.Now()
	tv.mu.Lock()
	stale := now.Sub(tv.touched) >= trackerTouchInterval
	if stale {
		tv.touched = now
	}
	tv.mu.Unlock()
	if stale {
		vt.ttl.Touch(key)
	}
}

// AddValue records a value for the key and reports whether the key is over the
// limit afterwards, so the value that takes it past the limit is reported too.
// Strings are recorded as-is, integers and floats in their decimal form, and
// every other type in its Go string form.
func (vt *ValueTracker) AddValue(key string, value any) bool {
	return vt.AddString(key, valueString(value))
}

// AddString is AddValue for a string value.
func (vt *ValueTracker) AddString(key string, s string) bool {
	tv := vt.entry(key)
	tv.mu.Lock()
	defer tv.mu.Unlock()

	if tv.overLimit {
		return true
	}
	if vt.maxValuesPerKey <= 0 {
		return false
	}
	if _, ok := tv.values[s]; ok {
		return false
	}
	if len(tv.values) >= vt.maxValuesPerKey {
		tv.overLimit = true
		tv.values = nil
		return true
	}
	tv.values[s] = struct{}{}
	return false
}

// MarkOverLimit flags the key as over the limit regardless of how many values
// have been seen for it.
func (vt *ValueTracker) MarkOverLimit(key string) {
	tv := vt.entry(key)
	tv.mu.Lock()
	defer tv.mu.Unlock()
	tv.overLimit = true
	tv.values = nil
}

// IsOverLimit reports whether the key has exceeded the distinct-value limit or
// was flagged with MarkOverLimit within the TTL window.
func (vt *ValueTracker) IsOverLimit(key string) bool {
	item := vt.ttl.Get(key)
	if item == nil {
		return false
	}
	tv := item.Value()
	vt.touch(key, tv)
	tv.mu.Lock()
	defer tv.mu.Unlock()
	return tv.overLimit
}

func (vt *ValueTracker) Close() {
	vt.ttl.Stop()
}

func valueString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case int64:
		return strconv.FormatInt(v, 10)
	case int:
		return strconv.Itoa(v)
	case float64:
		return strconv.FormatFloat(v, 'g', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	default:
		return fmt.Sprint(v)
	}
}

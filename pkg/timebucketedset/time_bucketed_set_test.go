package timebucketedset

import (
	"sync"
	"testing"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSet(t *testing.T, width time.Duration, maxBuckets int, preWriteWindow time.Duration) *Set {
	t.Helper()
	set, err := New(width, Config{MaxBuckets: maxBuckets, MaxBucketSize: 32 << 20, PreWriteWindow: preWriteWindow})
	require.NoError(t, err)
	return set
}

func TestBucketStart(t *testing.T) {
	hour := time.Date(2026, 9, 22, 3, 0, 0, 0, time.UTC).UnixMilli()
	testCases := []struct {
		name      string
		width     time.Duration
		unixMilli int64
		want      int64
	}{
		{name: "ExactBoundary_Unchanged", width: time.Hour, unixMilli: hour, want: hour},
		{name: "MidBucket_RoundsDown", width: time.Hour, unixMilli: hour + 20*time.Minute.Milliseconds() + 123, want: hour},
		{name: "OneMillisecondBeforeBoundary_StaysInBucket", width: time.Hour, unixMilli: hour + time.Hour.Milliseconds() - 1, want: hour},
		{name: "HalfHourWidth_FirstHalf_RoundsToHour", width: 30 * time.Minute, unixMilli: hour + 29*time.Minute.Milliseconds(), want: hour},
		{name: "HalfHourWidth_SecondHalf_RoundsToHalfHour", width: 30 * time.Minute, unixMilli: hour + 31*time.Minute.Milliseconds(), want: hour + 30*time.Minute.Milliseconds()},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			set := newSet(t, testCase.width, 2, 0)
			assert.Equal(t, testCase.want, set.BucketStart(testCase.unixMilli))
		})
	}
}

func TestPlan_Registration(t *testing.T) {
	base := time.Date(2026, 9, 22, 4, 0, 0, 0, time.UTC).UnixMilli()
	hour := time.Hour.Milliseconds()
	series := []byte{0xa1, 0xb2, 0xc3, 0xd4, 0xe5, 0xf6, 0x07, 0x18, 0}
	reduced := []byte{0xa1, 0xb2, 0xc3, 0xd4, 0xe5, 0xf6, 0x07, 0x18, 1}
	testCases := []struct {
		name  string
		steps []Step
	}{
		{
			name:  "FirstSighting_CurTrue_NextFalse",
			steps: []Step{PlanStep(series, base, base+1_000, ExpectedBool(true), ExpectedBool(false))},
		},
		{
			name: "NotApplied_ReplannedOnNextCall",
			steps: []Step{
				PlanStep(series, base, base+1_000, ExpectedBool(true), ExpectedBool(false)),
				PlanStep(series, base, base+2_000, ExpectedBool(true), ExpectedBool(false)),
			},
		},
		{
			name: "Applied_NotReplanned",
			steps: []Step{
				PlanStep(series, base, base+1_000, ExpectedBool(true), ExpectedBool(false)),
				ApplyStep(series, base),
				PlanStep(series, base, base+2_000, ExpectedBool(false), ExpectedBool(false)),
			},
		},
		{
			name: "DistinctIds_Independent",
			steps: []Step{
				PlanStep(series, base, base+1_000, ExpectedBool(true), ExpectedBool(false)),
				ApplyStep(series, base),
				PlanStep(reduced, base, base+2_000, ExpectedBool(true), ExpectedBool(false)),
			},
		},
		{
			name: "SameId_DifferentBucket_Independent",
			steps: []Step{
				PlanStep(series, base, base+1_000, ExpectedBool(true), ExpectedBool(false)),
				ApplyStep(series, base),
				PlanStep(series, base+hour, base+hour+1_000, ExpectedBool(true), ExpectedBool(false)),
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			require.NoError(t, RunSteps(newSet(t, time.Hour, 3, 0), testCase.steps))
		})
	}
}

func TestApply_CopiesId(t *testing.T) {
	base := time.Date(2026, 9, 22, 4, 0, 0, 0, time.UTC).UnixMilli()
	set := newSet(t, time.Hour, 3, 0)
	buffer := []byte{7, 7, 7}
	cur, _ := set.Plan(buffer, base, base+1_000)
	require.True(t, cur)

	items := &Items{}
	items.Add(buffer, base)
	buffer[0] = 0
	set.Apply(items)

	cur, _ = set.Plan([]byte{7, 7, 7}, base, base+2_000)
	assert.False(t, cur, "the id as added must be registered")
	cur, _ = set.Plan([]byte{0, 7, 7}, base, base+2_000)
	assert.True(t, cur, "the mutated buffer must not be registered")
}

func TestApply_EmptiesItemsForReuse(t *testing.T) {
	base := time.Date(2026, 9, 22, 4, 0, 0, 0, time.UTC).UnixMilli()
	set := newSet(t, time.Hour, 3, 0)
	first, second := []byte{1}, []byte{2}
	set.Plan(first, base, base+1_000)

	items := &Items{}
	items.Add(first, base)
	items.Add(first, base)
	require.Equal(t, 2, items.Len())
	set.Apply(items)
	assert.Equal(t, 0, items.Len())

	items.Add(second, base)
	set.Apply(items)
	cur, _ := set.Plan(second, base, base+2_000)
	assert.False(t, cur)
}

func TestBuckets_Lifecycle(t *testing.T) {
	base := time.Date(2026, 9, 22, 4, 0, 0, 0, time.UTC).UnixMilli()
	hour := time.Hour.Milliseconds()
	bucket := func(n int64) int64 { return base + n*hour }
	a, b, c, d := []byte{0xaa}, []byte{0xbb}, []byte{0xcc}, []byte{0xdd}
	testCases := []struct {
		name            string
		maxBuckets      int
		steps           []Step
		wantLiveBuckets int
	}{
		{
			name:       "NewerBucket_WhenFull_EvictsOldest",
			maxBuckets: 2,
			steps: []Step{
				PlanStep(a, bucket(0), bucket(0)+1, ExpectedBool(true), ExpectedBool(false)),
				ApplyStep(a, bucket(0)),
				PlanStep(b, bucket(1), bucket(1)+1, ExpectedBool(true), ExpectedBool(false)),
				ApplyStep(b, bucket(1)),
				PlanStep(c, bucket(2), bucket(2)+1, ExpectedBool(true), ExpectedBool(false)),
				PlanStep(b, bucket(1), bucket(2)+2, ExpectedBool(false), ExpectedBool(false)),
				PlanStep(a, bucket(0), bucket(2)+3, ExpectedBool(true), ExpectedBool(false)),
			},
			wantLiveBuckets: 2,
		},
		{
			name:       "EvictedBucket_ForgetsItsIds",
			maxBuckets: 2,
			steps: []Step{
				PlanStep(a, bucket(0), bucket(0)+1, ExpectedBool(true), ExpectedBool(false)),
				ApplyStep(a, bucket(0)),
				PlanStep(b, bucket(1), bucket(1)+1, ExpectedBool(true), ExpectedBool(false)),
				PlanStep(c, bucket(2), bucket(2)+1, ExpectedBool(true), ExpectedBool(false)),
				PlanStep(d, bucket(3), bucket(3)+1, ExpectedBool(true), ExpectedBool(false)),
				PlanStep(a, bucket(3), bucket(3)+2, ExpectedBool(true), ExpectedBool(false)),
			},
			wantLiveBuckets: 2,
		},
		{
			name:       "OlderThanAllLive_WhenFull_Uncacheable",
			maxBuckets: 2,
			steps: []Step{
				PlanStep(a, bucket(1), bucket(1)+1, ExpectedBool(true), ExpectedBool(false)),
				PlanStep(b, bucket(2), bucket(2)+1, ExpectedBool(true), ExpectedBool(false)),
				PlanStep(c, bucket(0), bucket(2)+2, ExpectedBool(true), ExpectedBool(false)),
				ApplyStep(c, bucket(0)),
				PlanStep(c, bucket(0), bucket(2)+3, ExpectedBool(true), ExpectedBool(false)),
			},
			wantLiveBuckets: 2,
		},
		{
			name:       "OlderThanAllLive_WhenNotFull_Cacheable",
			maxBuckets: 3,
			steps: []Step{
				PlanStep(a, bucket(1), bucket(1)+1, ExpectedBool(true), ExpectedBool(false)),
				PlanStep(b, bucket(0), bucket(1)+2, ExpectedBool(true), ExpectedBool(false)),
				ApplyStep(b, bucket(0)),
				PlanStep(b, bucket(0), bucket(1)+3, ExpectedBool(false), ExpectedBool(false)),
			},
			wantLiveBuckets: 2,
		},
		{
			name:       "FutureBeyondOneWidth_Uncacheable",
			maxBuckets: 3,
			steps: []Step{
				PlanStep(a, bucket(2), bucket(0)+1, ExpectedBool(true), ExpectedBool(false)),
				ApplyStep(a, bucket(2)),
				PlanStep(a, bucket(2), bucket(0)+2, ExpectedBool(true), ExpectedBool(false)),
			},
			wantLiveBuckets: 0,
		},
		{
			name:       "FutureWithinOneWidth_Cacheable",
			maxBuckets: 3,
			steps: []Step{
				PlanStep(a, bucket(1), bucket(0)+1, ExpectedBool(true), ExpectedBool(false)),
				ApplyStep(a, bucket(1)),
				PlanStep(a, bucket(1), bucket(0)+2, ExpectedBool(false), ExpectedBool(false)),
			},
			wantLiveBuckets: 1,
		},
		{
			name:       "JumpAhead_EvictsOnePerNewBucket",
			maxBuckets: 2,
			steps: []Step{
				PlanStep(a, bucket(0), bucket(0)+1, ExpectedBool(true), ExpectedBool(false)),
				PlanStep(b, bucket(1), bucket(1)+1, ExpectedBool(true), ExpectedBool(false)),
				ApplyStep(b, bucket(1)),
				PlanStep(c, bucket(5), bucket(5)+1, ExpectedBool(true), ExpectedBool(false)),
				PlanStep(b, bucket(1), bucket(5)+2, ExpectedBool(false), ExpectedBool(false)),
				PlanStep(a, bucket(0), bucket(5)+3, ExpectedBool(true), ExpectedBool(false)),
			},
			wantLiveBuckets: 2,
		},
		{
			name:       "Apply_ForBucketNeverCreated_Dropped",
			maxBuckets: 3,
			steps: []Step{
				ApplyStep(a, bucket(0)),
				PlanStep(a, bucket(0), bucket(0)+1, ExpectedBool(true), ExpectedBool(false)),
			},
			wantLiveBuckets: 1,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			set := newSet(t, time.Hour, testCase.maxBuckets, 0)
			require.NoError(t, RunSteps(set, testCase.steps))
			assert.Len(t, set.buckets, testCase.wantLiveBuckets)
		})
	}
}

// Eviction hands the old bucket's cache to the new bucket instead of
// allocating another 32 MiB instance.
func TestBuckets_EvictedCacheIsReused(t *testing.T) {
	base := time.Date(2026, 9, 22, 4, 0, 0, 0, time.UTC).UnixMilli()
	hour := time.Hour.Milliseconds()
	set := newSet(t, time.Hour, 2, 0)
	id := []byte{0x42}

	set.Plan(id, base, base+1)
	set.Plan(id, base+hour, base+hour+1)
	oldest := set.buckets[base]
	require.NotNil(t, oldest)

	set.Plan(id, base+2*hour, base+2*hour+1)
	assert.Same(t, oldest, set.buckets[base+2*hour])
	assert.Empty(t, set.spareBuckets)
}

func TestPreWrite(t *testing.T) {
	base := time.Date(2026, 9, 22, 4, 0, 0, 0, time.UTC).UnixMilli()
	hour := time.Hour.Milliseconds()
	window := 10 * time.Minute
	windowStart := base + hour - window.Milliseconds()
	id := []byte{0x51, 0x6e, 0x30, 0x7a}
	// slots are deterministic in the id, so pick one id early and one late in the window
	slot := func(id []byte) int64 { return int64(xxhash.Sum64(id) % uint64(window.Milliseconds())) }
	var early, late []byte
	for i := 0; i < 256 && (early == nil || late == nil); i++ {
		candidate := []byte{byte(i), 0x99}
		switch {
		case early == nil && slot(candidate) < 2*time.Minute.Milliseconds():
			early = candidate
		case late == nil && slot(candidate) > 8*time.Minute.Milliseconds():
			late = candidate
		}
	}
	require.NotNil(t, early)
	require.NotNil(t, late)

	testCases := []struct {
		name           string
		maxBuckets     int
		preWriteWindow time.Duration
		steps          []Step
	}{
		{
			name:           "ZeroWindow_NeverPlansNext",
			maxBuckets:     3,
			preWriteWindow: 0,
			steps: []Step{
				PlanStep(id, base, base+1, ExpectedBool(true), ExpectedBool(false)),
				ApplyStep(id, base),
				PlanStep(id, base, base+hour-1, ExpectedBool(false), ExpectedBool(false)),
			},
		},
		{
			name:           "UnregisteredId_NoNext",
			maxBuckets:     3,
			preWriteWindow: window,
			steps:          []Step{PlanStep(id, base, base+hour-1, ExpectedBool(true), ExpectedBool(false))},
		},
		{
			name:           "BeforeWindowStart_NoNext",
			maxBuckets:     3,
			preWriteWindow: window,
			steps: []Step{
				PlanStep(id, base, base+1, ExpectedBool(true), ExpectedBool(false)),
				ApplyStep(id, base),
				PlanStep(id, base, windowStart-1, ExpectedBool(false), ExpectedBool(false)),
			},
		},
		{
			name:           "AtBucketEnd_AllIdsEligible",
			maxBuckets:     3,
			preWriteWindow: window,
			steps: []Step{
				PlanStep(late, base, base+1, ExpectedBool(true), ExpectedBool(false)),
				ApplyStep(late, base),
				PlanStep(late, base, base+hour-1, ExpectedBool(false), ExpectedBool(true)),
			},
		},
		{
			name:           "Staggered_ByIdHash",
			maxBuckets:     3,
			preWriteWindow: window,
			steps: []Step{
				PlanStep(early, base, base+1, ExpectedBool(true), ExpectedBool(false)),
				ApplyStep(early, base),
				PlanStep(late, base, base+2, ExpectedBool(true), ExpectedBool(false)),
				ApplyStep(late, base),
				PlanStep(early, base, windowStart+5*time.Minute.Milliseconds(), ExpectedBool(false), ExpectedBool(true)),
				PlanStep(late, base, windowStart+5*time.Minute.Milliseconds(), ExpectedBool(false), ExpectedBool(false)),
			},
		},
		{
			name:           "Applied_NextNotReplanned",
			maxBuckets:     3,
			preWriteWindow: window,
			steps: []Step{
				PlanStep(id, base, base+1, ExpectedBool(true), ExpectedBool(false)),
				ApplyStep(id, base),
				PlanStep(id, base, base+hour-1, ExpectedBool(false), ExpectedBool(true)),
				ApplyStep(id, base+hour),
				PlanStep(id, base, base+hour-1, ExpectedBool(false), ExpectedBool(false)),
			},
		},
		{
			name:           "PreWritten_SuppressesCurInNextBucket",
			maxBuckets:     3,
			preWriteWindow: window,
			steps: []Step{
				PlanStep(id, base, base+1, ExpectedBool(true), ExpectedBool(false)),
				ApplyStep(id, base),
				PlanStep(id, base, base+hour-1, ExpectedBool(false), ExpectedBool(true)),
				ApplyStep(id, base+hour),
				PlanStep(id, base+hour, base+hour+1, ExpectedBool(false), ExpectedBool(false)),
			},
		},
		{
			name:           "LaggingData_NoPreWrite",
			maxBuckets:     3,
			preWriteWindow: window,
			steps: []Step{
				PlanStep(id, base, base+1, ExpectedBool(true), ExpectedBool(false)),
				ApplyStep(id, base),
				PlanStep(id, base, base+hour+5*time.Minute.Milliseconds(), ExpectedBool(false), ExpectedBool(false)),
			},
		},
		{
			name:           "NextBucketCreation_EvictsOldest_WhenFull",
			maxBuckets:     2,
			preWriteWindow: window,
			steps: []Step{
				PlanStep(early, base-hour, base-hour+1, ExpectedBool(true), ExpectedBool(false)),
				ApplyStep(early, base-hour),
				PlanStep(id, base, base+1, ExpectedBool(true), ExpectedBool(false)),
				ApplyStep(id, base),
				PlanStep(id, base, base+hour-1, ExpectedBool(false), ExpectedBool(true)),
				PlanStep(early, base-hour, base+hour-1, ExpectedBool(true), ExpectedBool(false)),
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			require.NoError(t, RunSteps(newSet(t, time.Hour, testCase.maxBuckets, testCase.preWriteWindow), testCase.steps))
		})
	}
}

// A Plan for an id in a bucket it was never applied to must be true even while
// buckets are evicted and their caches reused underneath it; this is what the
// read lock spanning every fastcache call guarantees.
func TestConcurrent_PlanApply_NoStaleRegistration(t *testing.T) {
	base := time.Date(2026, 9, 22, 4, 0, 0, 0, time.UTC).UnixMilli()
	hour := time.Hour.Milliseconds()
	set := newSet(t, time.Hour, 2, 0)

	stop := make(chan struct{})
	var advancer sync.WaitGroup
	advancer.Add(1)
	go func() {
		defer advancer.Done()
		for n := int64(1); ; n++ {
			select {
			case <-stop:
				return
			default:
			}
			set.Plan([]byte{0xff, 0xff}, base+n*hour, base+n*hour+1)
		}
	}()

	const workers, rounds = 8, 400
	var work sync.WaitGroup
	for w := 0; w < workers; w++ {
		work.Add(1)
		go func(id []byte) {
			defer work.Done()
			items := &Items{}
			for n := int64(0); n < rounds; n++ {
				bucket := base + n*hour
				cur, _ := set.Plan(id, bucket, bucket+1)
				assert.True(t, cur, "worker %v: bucket %d planned as registered before any apply", id, n)
				items.Add(id, bucket)
				set.Apply(items)
			}
		}([]byte{byte(w), 0x01})
	}
	work.Wait()
	close(stop)
	advancer.Wait()
}

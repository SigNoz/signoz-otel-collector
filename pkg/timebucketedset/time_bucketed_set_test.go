package timebucketedset

import (
	"sync"
	"testing"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// step is one Plan or Apply call in a scenario; a table of steps reads as
// "plan, apply, plan again" and carries the expected Plan answers.
type step struct {
	op          string
	id          []byte
	bucketStart int64
	now         int64
	wantCur     bool
	wantNext    bool
}

func plan(id []byte, bucketStart, now int64, wantCur, wantNext bool) step {
	return step{op: "plan", id: id, bucketStart: bucketStart, now: now, wantCur: wantCur, wantNext: wantNext}
}

func apply(id []byte, bucketStart int64) step {
	return step{op: "apply", id: id, bucketStart: bucketStart}
}

func newSet(t *testing.T, width time.Duration, maxBuckets int, preWriteWindow time.Duration) *Set {
	t.Helper()
	set, err := New(width, Config{MaxBuckets: maxBuckets, MaxBucketSize: 32 << 20, PreWriteWindow: preWriteWindow})
	require.NoError(t, err)
	return set
}

func runSteps(t *testing.T, set *Set, steps []step) {
	t.Helper()
	items := &Items{}
	for i, s := range steps {
		switch s.op {
		case "plan":
			cur, next := set.Plan(s.id, s.bucketStart, s.now)
			assert.Equal(t, s.wantCur, cur, "step %d: cur", i)
			assert.Equal(t, s.wantNext, next, "step %d: next", i)
		case "apply":
			items.Add(s.id, s.bucketStart)
			set.Apply(items)
		default:
			t.Fatalf("step %d: unknown op %q", i, s.op)
		}
	}
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
		steps []step
	}{
		{
			name:  "FirstSighting_CurTrue_NextFalse",
			steps: []step{plan(series, base, base+1_000, true, false)},
		},
		{
			name: "NotApplied_ReplannedOnNextCall",
			steps: []step{
				plan(series, base, base+1_000, true, false),
				plan(series, base, base+2_000, true, false),
			},
		},
		{
			name: "Applied_NotReplanned",
			steps: []step{
				plan(series, base, base+1_000, true, false),
				apply(series, base),
				plan(series, base, base+2_000, false, false),
			},
		},
		{
			name: "DistinctIds_Independent",
			steps: []step{
				plan(series, base, base+1_000, true, false),
				apply(series, base),
				plan(reduced, base, base+2_000, true, false),
			},
		},
		{
			name: "SameId_DifferentBucket_Independent",
			steps: []step{
				plan(series, base, base+1_000, true, false),
				apply(series, base),
				plan(series, base+hour, base+hour+1_000, true, false),
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			runSteps(t, newSet(t, time.Hour, 3, 0), testCase.steps)
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
		steps           []step
		wantLiveBuckets int
	}{
		{
			name:       "NewerBucket_WhenFull_EvictsOldest",
			maxBuckets: 2,
			steps: []step{
				plan(a, bucket(0), bucket(0)+1, true, false),
				apply(a, bucket(0)),
				plan(b, bucket(1), bucket(1)+1, true, false),
				apply(b, bucket(1)),
				plan(c, bucket(2), bucket(2)+1, true, false),
				plan(b, bucket(1), bucket(2)+2, false, false),
				plan(a, bucket(0), bucket(2)+3, true, false),
			},
			wantLiveBuckets: 2,
		},
		{
			name:       "EvictedBucket_ForgetsItsIds",
			maxBuckets: 2,
			steps: []step{
				plan(a, bucket(0), bucket(0)+1, true, false),
				apply(a, bucket(0)),
				plan(b, bucket(1), bucket(1)+1, true, false),
				plan(c, bucket(2), bucket(2)+1, true, false),
				plan(d, bucket(3), bucket(3)+1, true, false),
				plan(a, bucket(3), bucket(3)+2, true, false),
			},
			wantLiveBuckets: 2,
		},
		{
			name:       "OlderThanAllLive_WhenFull_Uncacheable",
			maxBuckets: 2,
			steps: []step{
				plan(a, bucket(1), bucket(1)+1, true, false),
				plan(b, bucket(2), bucket(2)+1, true, false),
				plan(c, bucket(0), bucket(2)+2, true, false),
				apply(c, bucket(0)),
				plan(c, bucket(0), bucket(2)+3, true, false),
			},
			wantLiveBuckets: 2,
		},
		{
			name:       "OlderThanAllLive_WhenNotFull_Cacheable",
			maxBuckets: 3,
			steps: []step{
				plan(a, bucket(1), bucket(1)+1, true, false),
				plan(b, bucket(0), bucket(1)+2, true, false),
				apply(b, bucket(0)),
				plan(b, bucket(0), bucket(1)+3, false, false),
			},
			wantLiveBuckets: 2,
		},
		{
			name:       "FutureBeyondOneWidth_Uncacheable",
			maxBuckets: 3,
			steps: []step{
				plan(a, bucket(2), bucket(0)+1, true, false),
				apply(a, bucket(2)),
				plan(a, bucket(2), bucket(0)+2, true, false),
			},
			wantLiveBuckets: 0,
		},
		{
			name:       "FutureWithinOneWidth_Cacheable",
			maxBuckets: 3,
			steps: []step{
				plan(a, bucket(1), bucket(0)+1, true, false),
				apply(a, bucket(1)),
				plan(a, bucket(1), bucket(0)+2, false, false),
			},
			wantLiveBuckets: 1,
		},
		{
			name:       "JumpAhead_EvictsOnePerNewBucket",
			maxBuckets: 2,
			steps: []step{
				plan(a, bucket(0), bucket(0)+1, true, false),
				plan(b, bucket(1), bucket(1)+1, true, false),
				apply(b, bucket(1)),
				plan(c, bucket(5), bucket(5)+1, true, false),
				plan(b, bucket(1), bucket(5)+2, false, false),
				plan(a, bucket(0), bucket(5)+3, true, false),
			},
			wantLiveBuckets: 2,
		},
		{
			name:       "Apply_ForBucketNeverCreated_Dropped",
			maxBuckets: 3,
			steps: []step{
				apply(a, bucket(0)),
				plan(a, bucket(0), bucket(0)+1, true, false),
			},
			wantLiveBuckets: 1,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			set := newSet(t, time.Hour, testCase.maxBuckets, 0)
			runSteps(t, set, testCase.steps)
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
		steps          []step
	}{
		{
			name:           "ZeroWindow_NeverPlansNext",
			maxBuckets:     3,
			preWriteWindow: 0,
			steps: []step{
				plan(id, base, base+1, true, false),
				apply(id, base),
				plan(id, base, base+hour-1, false, false),
			},
		},
		{
			name:           "UnregisteredId_NoNext",
			maxBuckets:     3,
			preWriteWindow: window,
			steps:          []step{plan(id, base, base+hour-1, true, false)},
		},
		{
			name:           "BeforeWindowStart_NoNext",
			maxBuckets:     3,
			preWriteWindow: window,
			steps: []step{
				plan(id, base, base+1, true, false),
				apply(id, base),
				plan(id, base, windowStart-1, false, false),
			},
		},
		{
			name:           "AtBucketEnd_AllIdsEligible",
			maxBuckets:     3,
			preWriteWindow: window,
			steps: []step{
				plan(late, base, base+1, true, false),
				apply(late, base),
				plan(late, base, base+hour-1, false, true),
			},
		},
		{
			name:           "Staggered_ByIdHash",
			maxBuckets:     3,
			preWriteWindow: window,
			steps: []step{
				plan(early, base, base+1, true, false),
				apply(early, base),
				plan(late, base, base+2, true, false),
				apply(late, base),
				plan(early, base, windowStart+5*time.Minute.Milliseconds(), false, true),
				plan(late, base, windowStart+5*time.Minute.Milliseconds(), false, false),
			},
		},
		{
			name:           "Applied_NextNotReplanned",
			maxBuckets:     3,
			preWriteWindow: window,
			steps: []step{
				plan(id, base, base+1, true, false),
				apply(id, base),
				plan(id, base, base+hour-1, false, true),
				apply(id, base+hour),
				plan(id, base, base+hour-1, false, false),
			},
		},
		{
			name:           "PreWritten_SuppressesCurInNextBucket",
			maxBuckets:     3,
			preWriteWindow: window,
			steps: []step{
				plan(id, base, base+1, true, false),
				apply(id, base),
				plan(id, base, base+hour-1, false, true),
				apply(id, base+hour),
				plan(id, base+hour, base+hour+1, false, false),
			},
		},
		{
			name:           "LaggingData_NoPreWrite",
			maxBuckets:     3,
			preWriteWindow: window,
			steps: []step{
				plan(id, base, base+1, true, false),
				apply(id, base),
				plan(id, base, base+hour+5*time.Minute.Milliseconds(), false, false),
			},
		},
		{
			name:           "NextBucketCreation_EvictsOldest_WhenFull",
			maxBuckets:     2,
			preWriteWindow: window,
			steps: []step{
				plan(early, base-hour, base-hour+1, true, false),
				apply(early, base-hour),
				plan(id, base, base+1, true, false),
				apply(id, base),
				plan(id, base, base+hour-1, false, true),
				plan(early, base-hour, base+hour-1, true, false),
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			runSteps(t, newSet(t, time.Hour, testCase.maxBuckets, testCase.preWriteWindow), testCase.steps)
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

func BenchmarkPlan_Hit(b *testing.B) {
	set, err := New(time.Hour, Config{MaxBuckets: 3, MaxBucketSize: 32 << 20})
	if err != nil {
		b.Fatal(err)
	}
	base := time.Date(2026, 9, 22, 4, 0, 0, 0, time.UTC).UnixMilli()
	id := []byte{1, 2, 3, 4, 5, 6, 7, 8, 0}
	set.Plan(id, base, base+1)
	items := &Items{}
	items.Add(id, base)
	set.Apply(items)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if cur, _ := set.Plan(id, base, base+1); cur {
			b.Fatal("registered id planned again")
		}
	}
}

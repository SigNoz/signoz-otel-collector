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

func TestPlanAndApply(t *testing.T) {
	baseUnixMilliseconds := time.Date(2026, 9, 22, 4, 0, 0, 0, time.UTC).UnixMilli()
	width := time.Hour

	id := []byte{0xa1, 0xb2, 0xc3, 0xd4, 0xe5, 0xf6, 0x07, 0x18, 0}
	differentId := []byte{0xa1, 0xb2, 0xc3, 0xd4, 0xe5, 0xf6, 0x07, 0x18, 1}

	testCases := []struct {
		name  string
		steps []Step
	}{
		{
			name: "FirstSighting_CurrentTrue_NextFalse",
			steps: []Step{
				PlanStep(id, baseUnixMilliseconds, baseUnixMilliseconds+1_000, ExpectedBool(true), ExpectedBool(false)),
			},
		},
		{
			name: "NotApplied_ReplannedOnNextCall",
			steps: []Step{
				PlanStep(id, baseUnixMilliseconds, baseUnixMilliseconds+1_000, ExpectedBool(true), ExpectedBool(false)),
				PlanStep(id, baseUnixMilliseconds, baseUnixMilliseconds+2_000, ExpectedBool(true), ExpectedBool(false)),
			},
		},
		{
			name: "Applied_NotReplanned",
			steps: []Step{
				PlanStep(id, baseUnixMilliseconds, baseUnixMilliseconds+1_000, ExpectedBool(true), ExpectedBool(false)),
				ApplyStep(id, baseUnixMilliseconds),
				PlanStep(id, baseUnixMilliseconds, baseUnixMilliseconds+2_000, ExpectedBool(false), ExpectedBool(false)),
			},
		},
		{
			name: "DistinctIds_Independent",
			steps: []Step{
				PlanStep(id, baseUnixMilliseconds, baseUnixMilliseconds+1_000, ExpectedBool(true), ExpectedBool(false)),
				ApplyStep(id, baseUnixMilliseconds),
				PlanStep(differentId, baseUnixMilliseconds, baseUnixMilliseconds+2_000, ExpectedBool(true), ExpectedBool(false)),
			},
		},
		{
			name: "SameId_DifferentBucket_Independent",
			steps: []Step{
				PlanStep(id, baseUnixMilliseconds, baseUnixMilliseconds+1_000, ExpectedBool(true), ExpectedBool(false)),
				ApplyStep(id, baseUnixMilliseconds),
				PlanStep(id, baseUnixMilliseconds+time.Hour.Milliseconds(), baseUnixMilliseconds+time.Hour.Milliseconds()+1_000, ExpectedBool(true), ExpectedBool(false)),
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			require.NoError(t, RunSteps(newSet(t, width, 3, 0), testCase.steps))
		})
	}
}

func TestApply_MarksEveryYieldedRow(t *testing.T) {
	baseUnixMilliseconds := time.Date(2026, 9, 22, 4, 0, 0, 0, time.UTC).UnixMilli()
	nextUnixMilliseconds := baseUnixMilliseconds + time.Hour.Milliseconds()
	set := newSet(t, time.Hour, 3, 0)
	first, second, third := []byte{1}, []byte{2}, []byte{3}
	set.Plan(first, baseUnixMilliseconds, baseUnixMilliseconds+1)
	set.Plan(third, nextUnixMilliseconds, nextUnixMilliseconds+1)

	set.Apply(func(yield func([]byte, int64) bool) {
		yield(first, baseUnixMilliseconds)
		yield(second, baseUnixMilliseconds)
		yield(third, nextUnixMilliseconds)
	})

	require.NoError(t, RunSteps(set, []Step{
		PlanStep(first, baseUnixMilliseconds, baseUnixMilliseconds+2, ExpectedBool(false), nil),
		PlanStep(second, baseUnixMilliseconds, baseUnixMilliseconds+2, ExpectedBool(false), nil),
		PlanStep(third, nextUnixMilliseconds, nextUnixMilliseconds+2, ExpectedBool(false), nil),
	}))
}

// The exporter builds each key into one scratch buffer per batch; Apply must
// have finished with an id before the next yield overwrites it.
func TestApply_KeyBufferReusedAcrossYields(t *testing.T) {
	baseUnixMilliseconds := time.Date(2026, 9, 22, 4, 0, 0, 0, time.UTC).UnixMilli()
	set := newSet(t, time.Hour, 3, 0)
	set.Plan([]byte{0}, baseUnixMilliseconds, baseUnixMilliseconds+1)

	var scratch [3]byte
	set.Apply(func(yield func([]byte, int64) bool) {
		for _, last := range []byte{7, 8, 9} {
			scratch = [3]byte{1, 2, last}
			if !yield(scratch[:], baseUnixMilliseconds) {
				return
			}
		}
	})

	require.NoError(t, RunSteps(set, []Step{
		PlanStep([]byte{1, 2, 7}, baseUnixMilliseconds, baseUnixMilliseconds+2, ExpectedBool(false), nil),
		PlanStep([]byte{1, 2, 8}, baseUnixMilliseconds, baseUnixMilliseconds+2, ExpectedBool(false), nil),
		PlanStep([]byte{1, 2, 9}, baseUnixMilliseconds, baseUnixMilliseconds+2, ExpectedBool(false), nil),
		PlanStep([]byte{1, 2, 0}, baseUnixMilliseconds, baseUnixMilliseconds+2, ExpectedBool(true), nil),
	}))
}

func TestBuckets_Lifecycle(t *testing.T) {
	baseUnixMilliseconds := time.Date(2026, 9, 22, 4, 0, 0, 0, time.UTC).UnixMilli()
	width := time.Hour

	nthBucket := func(n int64) int64 { return baseUnixMilliseconds + n*(width.Milliseconds()) }
	a, b, c, d := []byte{0xaa}, []byte{0xbb}, []byte{0xcc}, []byte{0xdd}

	testCases := []struct {
		name            string
		maxBuckets      int
		steps           []Step
		expectedBuckets int
	}{
		{
			name:       "NewerBucket_WhenFull_EvictsOldest",
			maxBuckets: 2,
			steps: []Step{
				PlanStep(a, nthBucket(0), nthBucket(0)+1, ExpectedBool(true), ExpectedBool(false)), // a added to 0th bucket
				ApplyStep(a, nthBucket(0)),
				PlanStep(b, nthBucket(1), nthBucket(1)+1, ExpectedBool(true), ExpectedBool(false)), // b added to 1st bucket
				ApplyStep(b, nthBucket(1)),
				PlanStep(c, nthBucket(2), nthBucket(2)+1, ExpectedBool(true), ExpectedBool(false)),  // c added to a new bucket (a's bucket was reassigned to c)
				PlanStep(b, nthBucket(1), nthBucket(2)+2, ExpectedBool(false), ExpectedBool(false)), // b still has a bucket, so current is false
				PlanStep(a, nthBucket(0), nthBucket(2)+3, ExpectedBool(true), ExpectedBool(false)),  // a's bucket was deleted, so current is true
			},
			expectedBuckets: 2,
		},
		{
			name:       "EvictedBucket_ForgetsItsIds",
			maxBuckets: 2,
			steps: []Step{
				PlanStep(a, nthBucket(0), nthBucket(0)+1, ExpectedBool(true), ExpectedBool(false)),
				ApplyStep(a, nthBucket(0)),
				PlanStep(b, nthBucket(1), nthBucket(1)+1, ExpectedBool(true), ExpectedBool(false)),
				PlanStep(c, nthBucket(2), nthBucket(2)+1, ExpectedBool(true), ExpectedBool(false)),
				PlanStep(d, nthBucket(3), nthBucket(3)+1, ExpectedBool(true), ExpectedBool(false)),
				PlanStep(a, nthBucket(3), nthBucket(3)+2, ExpectedBool(true), ExpectedBool(false)),
			},
			expectedBuckets: 2,
		},
		{
			name:       "OlderThanAllLive_WhenFull_Uncacheable",
			maxBuckets: 2,
			steps: []Step{
				PlanStep(a, nthBucket(1), nthBucket(1)+1, ExpectedBool(true), ExpectedBool(false)),
				PlanStep(b, nthBucket(2), nthBucket(2)+1, ExpectedBool(true), ExpectedBool(false)),
				PlanStep(c, nthBucket(0), nthBucket(2)+2, ExpectedBool(true), ExpectedBool(false)),
				ApplyStep(c, nthBucket(0)),
				PlanStep(c, nthBucket(0), nthBucket(2)+3, ExpectedBool(true), ExpectedBool(false)),
			},
			expectedBuckets: 2,
		},
		{
			name:       "OlderThanAllLive_WhenNotFull_Cacheable",
			maxBuckets: 3,
			steps: []Step{
				PlanStep(a, nthBucket(1), nthBucket(1)+1, ExpectedBool(true), ExpectedBool(false)),
				PlanStep(b, nthBucket(0), nthBucket(1)+2, ExpectedBool(true), ExpectedBool(false)),
				ApplyStep(b, nthBucket(0)),
				PlanStep(b, nthBucket(0), nthBucket(1)+3, ExpectedBool(false), ExpectedBool(false)),
			},
			expectedBuckets: 2,
		},
		{
			name:       "FutureBeyondOneWidth_Uncacheable",
			maxBuckets: 3,
			steps: []Step{
				PlanStep(a, nthBucket(2), nthBucket(0)+1, ExpectedBool(true), ExpectedBool(false)),
				ApplyStep(a, nthBucket(2)),
				PlanStep(a, nthBucket(2), nthBucket(0)+2, ExpectedBool(true), ExpectedBool(false)),
			},
			expectedBuckets: 0,
		},
		{
			name:       "FutureWithinOneWidth_Cacheable",
			maxBuckets: 3,
			steps: []Step{
				PlanStep(a, nthBucket(1), nthBucket(0)+1, ExpectedBool(true), ExpectedBool(false)),
				ApplyStep(a, nthBucket(1)),
				PlanStep(a, nthBucket(1), nthBucket(0)+2, ExpectedBool(false), ExpectedBool(false)),
			},
			expectedBuckets: 1,
		},
		{
			name:       "JumpAhead_EvictsOnePerNewBucket",
			maxBuckets: 2,
			steps: []Step{
				PlanStep(a, nthBucket(0), nthBucket(0)+1, ExpectedBool(true), ExpectedBool(false)),
				PlanStep(b, nthBucket(1), nthBucket(1)+1, ExpectedBool(true), ExpectedBool(false)),
				ApplyStep(b, nthBucket(1)),
				PlanStep(c, nthBucket(5), nthBucket(5)+1, ExpectedBool(true), ExpectedBool(false)),
				PlanStep(b, nthBucket(1), nthBucket(5)+2, ExpectedBool(false), ExpectedBool(false)),
				PlanStep(a, nthBucket(0), nthBucket(5)+3, ExpectedBool(true), ExpectedBool(false)),
			},
			expectedBuckets: 2,
		},
		{
			name:       "Apply_ForBucketNeverCreated_Dropped",
			maxBuckets: 3,
			steps: []Step{
				ApplyStep(a, nthBucket(0)),
				PlanStep(a, nthBucket(0), nthBucket(0)+1, ExpectedBool(true), ExpectedBool(false)),
			},
			expectedBuckets: 1,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			set := newSet(t, time.Hour, testCase.maxBuckets, 0)
			require.NoError(t, RunSteps(set, testCase.steps))
			assert.Len(t, set.buckets, testCase.expectedBuckets)
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
			for n := int64(0); n < rounds; n++ {
				bucket := base + n*hour
				cur, _ := set.Plan(id, bucket, bucket+1)
				assert.True(t, cur, "worker %v: bucket %d planned as registered before any apply", id, n)
				set.Apply(SingleRow(id, bucket))
			}
		}([]byte{byte(w), 0x01})
	}
	work.Wait()
	close(stop)
	advancer.Wait()
}

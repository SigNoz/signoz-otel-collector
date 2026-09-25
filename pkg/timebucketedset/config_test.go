package timebucketedset

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
)

func TestConfig_WithDefaults(t *testing.T) {
	testCases := []struct {
		name string
		in   Config
		want Config
	}{
		{
			name: "ZeroValues_TakeDefaults",
			in:   Config{},
			want: Config{MaxBuckets: 3, MaxBucketSize: 256 << 20},
		},
		{
			name: "ExplicitValues_Kept",
			in:   Config{MaxBuckets: 5, MaxBucketSize: 64 << 20, PreWriteWindow: 10 * time.Minute},
			want: Config{MaxBuckets: 5, MaxBucketSize: 64 << 20, PreWriteWindow: 10 * time.Minute},
		},
		{
			name: "PreWriteWindow_StaysZero",
			in:   Config{MaxBuckets: 2},
			want: Config{MaxBuckets: 2, MaxBucketSize: 256 << 20},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.want, testCase.in.WithDefaults())
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	testCases := []struct {
		name    string
		width   time.Duration
		config  Config
		wantErr string
	}{
		{
			name:    "NonPositiveWidth_Rejected",
			width:   0,
			config:  Config{MaxBuckets: 3, MaxBucketSize: 32 << 20},
			wantErr: "width must be positive",
		},
		{
			name:    "MaxBucketsOne_Rejected",
			width:   time.Hour,
			config:  Config{MaxBuckets: 1, MaxBucketSize: 32 << 20},
			wantErr: "max_buckets must be at least 2",
		},
		{
			name:   "MaxBucketsTwo_Accepted",
			width:  time.Hour,
			config: Config{MaxBuckets: 2, MaxBucketSize: 32 << 20},
		},
		{
			name:    "MaxBucketSizeBelowFastcacheFloor_Rejected",
			width:   time.Hour,
			config:  Config{MaxBuckets: 3, MaxBucketSize: 32<<20 - 1},
			wantErr: "max_bucket_size must be at least",
		},
		{
			name:    "PreWriteWindowNegative_Rejected",
			width:   time.Hour,
			config:  Config{MaxBuckets: 3, MaxBucketSize: 32 << 20, PreWriteWindow: -time.Second},
			wantErr: "shorter than the bucket width",
		},
		{
			name:    "PreWriteWindowEqualToWidth_Rejected",
			width:   time.Hour,
			config:  Config{MaxBuckets: 3, MaxBucketSize: 32 << 20, PreWriteWindow: time.Hour},
			wantErr: "shorter than the bucket width",
		},
		{
			name:    "PreWriteWindowUnderOneSecond_Rejected",
			width:   time.Hour,
			config:  Config{MaxBuckets: 3, MaxBucketSize: 32 << 20, PreWriteWindow: 500 * time.Millisecond},
			wantErr: "at least 1s",
		},
		{
			name:   "PreWriteWindowZero_Accepted",
			width:  time.Hour,
			config: Config{MaxBuckets: 3, MaxBucketSize: 32 << 20},
		},
		{
			name:   "PreWriteWindowInsideWidth_Accepted",
			width:  30 * time.Minute,
			config: Config{MaxBuckets: 3, MaxBucketSize: 32 << 20, PreWriteWindow: 5 * time.Minute},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.config.Validate(testCase.width)
			if testCase.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, testCase.wantErr)
		})
	}
}

func TestNew_AppliesDefaultsBeforeValidate(t *testing.T) {
	set, err := New(time.Hour, Config{}, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	assert.Equal(t, Config{MaxBuckets: 3, MaxBucketSize: 256 << 20}, set.config)
}

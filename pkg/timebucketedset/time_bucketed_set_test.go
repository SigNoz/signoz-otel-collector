package timebucketedset

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBucketSetPlan(t *testing.T) {
	config := Config{
		MaxBuckets:     2,
		MaxBucketSize:  defaultMaxBucketSize,
		PreWriteWindow: 0,
	}

	bs, err := New(1*time.Minute, config)
	require.NoError(t, err)

	id := []byte{123}

	bucketStart := bs.BucketStart(time.Now().UnixMilli())

	current, next := bs.Plan(id, bucketStart, time.Now().UnixMilli())
	assert.Equal(t, true, current)
	assert.Equal(t, false, next)

	bs.Apply(&Items{ids: [][]byte{id}, bucketKeys: []int64{bucketStart}})

	assert.True(t, bs.buckets[bucketStart].Has(id))

	current, next = bs.Plan(id, bucketStart, time.Now().UnixMilli())
	assert.Equal(t, false, current)
	assert.Equal(t, false, next)
}

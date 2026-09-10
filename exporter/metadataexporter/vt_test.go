package metadataexporter

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestValueTrackerFlagsAKeyPastTheDistinctValueLimit(t *testing.T) {
	tracker := NewValueTracker(10, 2, time.Minute)
	defer tracker.Close()

	assert.False(t, tracker.AddValue("http.route", "/users"))
	assert.False(t, tracker.AddValue("http.route", "/users"))
	assert.False(t, tracker.AddValue("http.route", "/orders"))
	assert.False(t, tracker.IsOverLimit("http.route"), "two distinct values are within the limit of two")

	assert.True(t, tracker.AddValue("http.route", "/carts"), "a third distinct value puts the key over the limit")
	assert.True(t, tracker.IsOverLimit("http.route"))
	assert.True(t, tracker.AddValue("http.route", "/users"), "an earlier value is over the limit too once the key is")

	tracker.AddValue("task.id", int64(1))
	tracker.AddValue("task.id", int64(1))
	tracker.AddValue("task.id", 1.0)
	assert.False(t, tracker.IsOverLimit("task.id"), "numbers are compared by their decimal form")

	tracker.MarkOverLimit("payload")
	assert.True(t, tracker.IsOverLimit("payload"), "a marked key is over the limit without any values")
	assert.False(t, tracker.IsOverLimit("unknown"))
}

func TestValueTrackerWithoutALimitOnlyFlagsMarkedKeys(t *testing.T) {
	tracker := NewValueTracker(10, 0, time.Minute)
	defer tracker.Close()

	for i := 0; i < 100; i++ {
		tracker.AddValue("http.route", fmt.Sprintf("/users/%d", i))
	}
	assert.False(t, tracker.IsOverLimit("http.route"))

	tracker.MarkOverLimit("http.route")
	assert.True(t, tracker.IsOverLimit("http.route"))
}
func BenchmarkValueTracker_RealisticLoad(b *testing.B) {
	const (
		numSpans         = 100_000
		batchSize        = 50_000
		totalKeys        = 1_000
		highCardinalKeys = 200 // 20% of keys
		normalValuePool  = 100
		normalUniqueVals = 30
	)

	tracker := NewValueTracker(1000, 10000, 45*time.Minute)
	defer tracker.Close()

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	keys := make([]string, totalKeys)
	highCardinalKeyMap := make(map[string]bool)

	for i := 0; i < highCardinalKeys; i++ {
		keys[i] = fmt.Sprintf("high-card-key-%d", i)
		highCardinalKeyMap[keys[i]] = true
	}

	for i := highCardinalKeys; i < totalKeys; i++ {
		keys[i] = fmt.Sprintf("key-%d", i)
	}

	valuePool := make(map[string][]string)
	for _, key := range keys {
		if !highCardinalKeyMap[key] {
			values := make([]string, normalValuePool)
			for i := 0; i < normalUniqueVals; i++ {
				values[i] = fmt.Sprintf("unique-value-%s-%d", key, i)
			}
			for i := normalUniqueVals; i < normalValuePool; i++ {
				values[i] = values[r.Intn(normalUniqueVals)]
			}
			valuePool[key] = values
		}
	}

	uniqueValueCounter := make(map[string]int)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for j := 0; j < numSpans; j += batchSize {
			batchSize := min(batchSize, numSpans-j)

			for k := 0; k < batchSize; k++ {
				numAttrs := 10 + r.Intn(41)
				for a := 0; a < numAttrs; a++ {
					key := keys[r.Intn(len(keys))]
					var value string

					if highCardinalKeyMap[key] {
						uniqueValueCounter[key]++
						value = fmt.Sprintf("unique-value-%s-%d", key, uniqueValueCounter[key])
					} else {
						values := valuePool[key]
						value = values[r.Intn(len(values))]
					}

					tracker.AddValue(key, value)
				}
			}

			for k := 0; k < 10; k++ {
				key := keys[r.Intn(len(keys))]
				tracker.IsOverLimit(key)
			}
		}
	}
}

func BenchmarkValueTracker_HighCardinality(b *testing.B) {
	tracker := NewValueTracker(1000, 100000, 45*time.Minute)
	defer tracker.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Add 100K unique values to a single key
		key := "high-card-key"
		for j := 0; j < 100000; j++ {
			tracker.AddValue(key, fmt.Sprintf("unique-value-%d", j))
		}
	}
}

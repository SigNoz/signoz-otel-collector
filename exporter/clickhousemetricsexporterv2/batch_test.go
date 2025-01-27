package clickhousemetricsexporterv2

import (
	"testing"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

type batchValue struct {
	samples []sample
	ts      []ts
}

type batchPointer struct {
	samples []*sample
	ts      []*ts
}

func generateSample() sample {
	return sample{
		env:         "production",
		temporality: pmetric.AggregationTemporalityCumulative,
		metricName:  "http_requests_total",
		fingerprint: 12345,
		unixMilli:   1632150000000,
		value:       123.45,
	}
}

func generateTs() ts {
	return ts{
		env:           "production",
		temporality:   pmetric.AggregationTemporalityCumulative,
		metricName:    "test",
		description:   "test",
		unit:          "test",
		isMonotonic:   false,
		attrs:         map[string]string{"a": "b"},
		scopeAttrs:    map[string]string{"a": "b"},
		resourceAttrs: map[string]string{"a": "b"},
	}
}

// BenchmarkGrowValue-1million    	       5	 232525983 ns/op	1149932720 B/op	      79 allocs/op
// BenchmarkGrowValue-10k 	     645	   1685683 ns/op	 8516823 B/op	      38 allocs/op
func BenchmarkGrowValue(b *testing.B) {
	s := generateSample()
	ts := generateTs()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		batch := batchValue{
			samples: make([]sample, 0),
		}
		for j := 0; j < 10000; j++ {
			batch.samples = append(batch.samples, s)
			batch.ts = append(batch.ts, ts)
		}
	}
}

// BenchmarkGrowPointer-1million    	      14	  75,508,500 ns/op	89897213 B/op (85.732663)	      76 allocs/op
// BenchmarkGrowPointer-10k   	   15375	     77279 ns/op	  620785 B/op	      36 allocs/op
func BenchmarkGrowPointer(b *testing.B) {
	s := generateSample()
	ts := generateTs()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		batch := batchPointer{
			samples: make([]*sample, 0),
		}
		for j := 0; j < 10000; j++ {
			batch.samples = append(batch.samples, &s)
			batch.ts = append(batch.ts, &ts)
		}
	}
}

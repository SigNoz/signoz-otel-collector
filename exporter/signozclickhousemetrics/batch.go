package signozclickhousemetrics

import (
	"time"

	pkgfingerprint "github.com/SigNoz/signoz-otel-collector/internal/common/fingerprint"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type batch struct {
	samples  []*sample
	expHist  []*exponentialHistogramSample
	ts       []*ts
	metadata []*metadata

	metaSeen map[string]struct{}
	metaIdx  map[string]int // maps seenKey -> index in metadata slice
}

func newBatch() *batch {
	return &batch{
		samples:  make([]*sample, 0),
		expHist:  make([]*exponentialHistogramSample, 0),
		ts:       make([]*ts, 0),
		metadata: make([]*metadata, 0),
		metaSeen: make(map[string]struct{}),
		metaIdx:  make(map[string]int),
	}
}

func (b *batch) addMetadata(name, desc, unit string, typ pmetric.MetricType, temporality pmetric.AggregationTemporality, isMonotonic bool, fingerprint *pkgfingerprint.Fingerprint, firstSeenUnixMilli, lastSeenUnixMilli *int64) {
	// Handle nil pointers - use current time as fallback
	var firstTimestamp, lastTimestamp int64
	if firstSeenUnixMilli != nil {
		// TODO(nikhilmantri0902, srikanthccv): This is a hack to handle the case where the first and last seen timestamps are not provided.
		// can happen when datapoints are 0 and we are here setting resource/scope attributes.
		// We should remove this once we have a proper way to handle this.
		// we can choose to skip adding metadata in this case.
		firstTimestamp = *firstSeenUnixMilli
		lastTimestamp = *lastSeenUnixMilli
	} else {
		now := time.Now().UnixMilli()
		firstTimestamp = now
		lastTimestamp = now
	}

	for key, value := range fingerprint.Attributes() {
		seenKey := key + name
		if key == "le" {
			seenKey += value.Val
		}
		// there should never be a conflicting key (either with resource, scope, or point attributes) in metrics
		// it breaks the fingerprinting, we assume this will never happen
		// even if it does, we will not handle it on our end (because we can't reliably which should take
		// precedence), the user should be responsible for ensuring no conflicting keys in their metrics

		// Check if metadata already exists
		if idx, exists := b.metaIdx[seenKey]; exists {
			// Update timestamps if provided
			if firstSeenUnixMilli != nil {
				existing := b.metadata[idx]
				if existing.firstReportedUnixMilli == 0 || firstTimestamp < existing.firstReportedUnixMilli {
					existing.firstReportedUnixMilli = firstTimestamp
				}
				if lastTimestamp > existing.lastReportedUnixMilli {
					existing.lastReportedUnixMilli = lastTimestamp
				}
			}
			continue
		}

		// New metadata entry
		b.metaSeen[seenKey] = struct{}{}
		idx := len(b.metadata)
		b.metaIdx[seenKey] = idx
		b.metadata = append(b.metadata, &metadata{
			metricName:             name,
			temporality:            temporality,
			description:            desc,
			unit:                   unit,
			typ:                    typ,
			isMonotonic:            isMonotonic,
			attrName:               key,
			attrType:               fingerprint.Type().String(),
			attrDatatype:           value.DataType,
			attrStringValue:        value.Val,
			firstReportedUnixMilli: firstTimestamp,
			lastReportedUnixMilli:  lastTimestamp,
		})
	}
}

func (b *batch) addSample(sample *sample) {
	b.samples = append(b.samples, sample)
}

func (b *batch) addTs(ts *ts) {
	b.ts = append(b.ts, ts)
}

func (b *batch) addExpHist(expHist *exponentialHistogramSample) {
	b.expHist = append(b.expHist, expHist)
}

// setFirstSeenLastSeen updates the first and last seen timestamps.
// If the pointers are nil, it allocates new memory and initializes them.
// Otherwise, it updates them to track the minimum and maximum values.
// Returns the updated pointers.
func setFirstSeenLastSeen(firstSeenUnixMilli, lastSeenUnixMilli *int64, unixMilli int64) (*int64, *int64) {
	if firstSeenUnixMilli == nil {
		firstSeenUnixMilli = new(int64)
		lastSeenUnixMilli = new(int64)
		*firstSeenUnixMilli = unixMilli
		*lastSeenUnixMilli = unixMilli
	} else {
		*firstSeenUnixMilli = min(*firstSeenUnixMilli, unixMilli)
		*lastSeenUnixMilli = max(*lastSeenUnixMilli, unixMilli)
	}
	return firstSeenUnixMilli, lastSeenUnixMilli
}

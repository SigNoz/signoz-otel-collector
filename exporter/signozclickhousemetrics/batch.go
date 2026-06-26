package signozclickhousemetrics

import (
	"math"
	"time"

	pkgfingerprint "github.com/SigNoz/signoz-otel-collector/internal/common/fingerprint"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type batch struct {
	samples  []sample
	expHist  []exponentialHistogramSample
	ts       []ts
	metadata []metadata
	// per-batch dedup of metadata rows by identity (metaKey)
	metaIdx map[metaKey]int
	logger  *zap.Logger
}

// newBatch pre-sizes each slice from a per-table hint (the previous batch's length).
func newBatch(logger *zap.Logger, samplesHint, tsHint, metadataHint int) *batch {
	return &batch{
		samples:  make([]sample, 0, max(samplesHint, 0)),
		expHist:  make([]exponentialHistogramSample, 0),
		ts:       make([]ts, 0, max(tsHint, 0)),
		metadata: make([]metadata, 0, max(metadataHint, 0)),
		metaIdx:  make(map[metaKey]int, max(metadataHint, 0)),
		logger:   logger,
	}
}

func (b *batch) addMetadata(name, desc, unit string, typ pmetric.MetricType, temporality pmetric.AggregationTemporality, isMonotonic bool, fingerprint *pkgfingerprint.Fingerprint, firstSeenUnixMilli, lastSeenUnixMilli int64) {

	if firstSeenUnixMilli == int64(math.MaxInt64) { // which means they were never set because of zero samples, default to now
		now := time.Now().UnixMilli()
		firstSeenUnixMilli = now
		lastSeenUnixMilli = now
		b.logger.Debug("firstSeen/lastSeen not provided; defaulting to now", zap.Int64("first_seen_unix_milli", firstSeenUnixMilli), zap.Int64("last_seen_unix_milli", lastSeenUnixMilli))
	}

	attrType := fingerprint.Type().String()
	for _, attr := range fingerprint.Attributes() {
		key, value := attr.Key, attr.Value
		mk := metaKey{
			temporality:     temporality,
			metricName:      name,
			attrName:        key,
			attrType:        attrType,
			attrDatatype:    value.DataType,
			attrStringValue: value.Val,
		}
		if idx, ok := b.metaIdx[mk]; ok {
			// already recorded this batch; widen its reported window (min first, max last)
			existing := &b.metadata[idx]
			if firstSeenUnixMilli < existing.firstReportedUnixMilli {
				existing.firstReportedUnixMilli = firstSeenUnixMilli
			}
			if lastSeenUnixMilli > existing.lastReportedUnixMilli {
				existing.lastReportedUnixMilli = lastSeenUnixMilli
			}
			continue
		}
		b.metaIdx[mk] = len(b.metadata)
		b.metadata = append(b.metadata, metadata{
			metricName:             name,
			temporality:            temporality,
			description:            desc,
			unit:                   unit,
			typ:                    typ,
			isMonotonic:            isMonotonic,
			attrName:               key,
			attrType:               attrType,
			attrDatatype:           value.DataType,
			attrStringValue:        value.Val,
			firstReportedUnixMilli: firstSeenUnixMilli,
			lastReportedUnixMilli:  lastSeenUnixMilli,
		})
	}
}

func (b *batch) addSample(sample *sample) {
	b.samples = append(b.samples, *sample)
}

func (b *batch) addTs(ts *ts) {
	b.ts = append(b.ts, *ts)
}

func (b *batch) addExpHist(expHist *exponentialHistogramSample) {
	b.expHist = append(b.expHist, *expHist)
}

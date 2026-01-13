package signozclickhousemetrics

import (
	"math"
	"time"

	pkgfingerprint "github.com/SigNoz/signoz-otel-collector/internal/common/fingerprint"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type batch struct {
	samples  []*sample
	expHist  []*exponentialHistogramSample
	ts       []*ts
	metadata []*metadata

	logger *zap.Logger
}

func newBatch(logger *zap.Logger) *batch {
	return &batch{
		samples:  make([]*sample, 0),
		expHist:  make([]*exponentialHistogramSample, 0),
		ts:       make([]*ts, 0),
		metadata: make([]*metadata, 0),
		logger:   logger,
	}
}

func (b *batch) addMetadata(name, desc, unit string, typ pmetric.MetricType, temporality pmetric.AggregationTemporality, isMonotonic bool, fingerprint *pkgfingerprint.Fingerprint, firstSeenUnixMilli, lastSeenUnixMilli int64) {

	if firstSeenUnixMilli == int64(math.MaxInt64) { // which means they were never set because of zero samples, default to now
		now := time.Now().UnixMilli()
		firstSeenUnixMilli = now
		lastSeenUnixMilli = now
		b.logger.Warn("firstSeen/lastSeen not provided; defaulting to now",
			zap.Int64("first_seen_unix_milli", firstSeenUnixMilli),
			zap.Int64("last_seen_unix_milli", lastSeenUnixMilli),
		)
	}

	// Append all metadata entries without deduplication.
	// The AggregatingMergeTree engine in ClickHouse will handle merging rows
	// with the same ORDER BY key (temporality, metric_name, attr_name, attr_type, attr_datatype, attr_string_value)
	// during background merges, aggregating first_reported_unix_milli (min) and last_reported_unix_milli (max).
	for key, value := range fingerprint.Attributes() {
		// there should never be a conflicting key (either with resource, scope, or point attributes) in metrics
		// it breaks the fingerprinting, we assume this will never happen
		// even if it does, we will not handle it on our end (because we can't reliably which should take
		// precedence), the user should be responsible for ensuring no conflicting keys in their metrics

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
			firstReportedUnixMilli: firstSeenUnixMilli,
			lastReportedUnixMilli:  lastSeenUnixMilli,
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

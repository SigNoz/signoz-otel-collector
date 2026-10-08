package signozclickhousemetrics

import (
	"math"
	"time"

	pkgfingerprint "github.com/SigNoz/signoz-otel-collector/internal/common/fingerprint"
	"github.com/SigNoz/signoz-otel-collector/pkg/timebucketedset"
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
	// per-batch dedup of time series rows: the time bucketed set is only marked
	// after a successful send, so repeats within one batch must be caught here
	tsSeen   map[tsKey]struct{}
	nowMilli int64
	// nil when the exporter deduplicates through the TTL cache instead
	timeSeriesTimeBucketedSet *timebucketedset.Set
	logger                    *zap.Logger
}

// tsKey identifies a time series row within one batch.
type tsKey struct {
	fingerprint uint64
	reduced     bool
	bucketStart int64
}

// newBatch pre-sizes each slice from a per-table hint (the previous batch's length).
func newBatch(logger *zap.Logger, timeSeriesTimeBucketedSet *timebucketedset.Set, samplesHint, tsHint, metadataHint int) *batch {
	return &batch{
		samples:                   make([]sample, 0, max(samplesHint, 0)),
		expHist:                   make([]exponentialHistogramSample, 0),
		ts:                        make([]ts, 0, max(tsHint, 0)),
		metadata:                  make([]metadata, 0, max(metadataHint, 0)),
		metaIdx:                   make(map[metaKey]int, max(metadataHint, 0)),
		tsSeen:                    make(map[tsKey]struct{}, max(tsHint, 0)),
		timeSeriesTimeBucketedSet: timeSeriesTimeBucketedSet,
		logger:                    logger,
	}
}

// seenTs records key and reports whether it was already recorded.
func (b *batch) seenTs(key tsKey) bool {
	if _, ok := b.tsSeen[key]; ok {
		return true
	}
	b.tsSeen[key] = struct{}{}
	return false
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

// setLabels builds the labels JSON and attribute maps, the expensive part of a row.
func (row *ts) setLabels(fingerprint *pkgfingerprint.Fingerprint, scopeAttrs, resourceAttrs map[string]string) {
	attrs := fingerprint.AttributesAsMap()
	row.labels = pkgfingerprint.NewLabelsAsJSONString(row.metricName, attrs, scopeAttrs, resourceAttrs)
	row.attrs = attrs
	row.scopeAttrs = scopeAttrs
	row.resourceAttrs = resourceAttrs
}

// planTimeSeries adds the time series rows one datapoint needs. row carries
// only scalar fields. Without the time bucketed set every row is built and the
// TTL cache decides at write time; with it, labels are built solely for rows
// that will be written, which in steady state is none.
func (b *batch) planTimeSeries(row ts, fingerprint *pkgfingerprint.Fingerprint, scopeAttrs, resourceAttrs map[string]string, reducer *reducer, reduced *reducedSeries) {
	row.bucketStart = row.unixMilli / timeSeriesBucket.Milliseconds() * timeSeriesBucket.Milliseconds()

	if b.timeSeriesTimeBucketedSet == nil {
		row.setLabels(fingerprint, scopeAttrs, resourceAttrs)
		b.addTs(&row)
		if reduced != nil && reducer.firstSeen(reduced.fingerprint) {
			reducedRow := reducedTsFrom(&row, reduced)
			b.addTs(&reducedRow)
		}
		return
	}

	var key [9]byte
	if !b.seenTs(tsKey{fingerprint: row.fingerprint, bucketStart: row.bucketStart}) {
		row.writeCurrent, row.writeNext = b.timeSeriesTimeBucketedSet.Plan(timeSeriesID(&key, row.fingerprint, false), row.bucketStart, b.nowMilli)
	}
	var reducedCurrent, reducedNext bool
	if reduced != nil && !b.seenTs(tsKey{fingerprint: reduced.fingerprint, reduced: true, bucketStart: row.bucketStart}) {
		reducedCurrent, reducedNext = b.timeSeriesTimeBucketedSet.Plan(timeSeriesID(&key, reduced.fingerprint, true), row.bucketStart, b.nowMilli)
	}

	if row.writeCurrent || row.writeNext {
		row.setLabels(fingerprint, scopeAttrs, resourceAttrs)
		b.addTs(&row)
	}
	if reducedCurrent || reducedNext {
		reducedRow := reducedTsFrom(&row, reduced)
		reducedRow.writeCurrent, reducedRow.writeNext = reducedCurrent, reducedNext
		b.addTs(&reducedRow)
	}
}

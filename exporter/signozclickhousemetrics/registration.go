package signozclickhousemetrics

import (
	"encoding/binary"
	"iter"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	pkgfingerprint "github.com/SigNoz/signoz-otel-collector/internal/common/fingerprint"
)

const (
	timeSeriesBucket      = time.Hour
	defaultPreWriteWindow = 15 * time.Minute
)

// tsKey identifies a registration row within one batch.
type tsKey struct {
	fingerprint uint64
	reduced     bool
	bucketStart int64
}

// seriesID packs a fingerprint and its reduced flag into key. A series whose
// rule drops nothing is its own reduction, so raw and reduced rows need
// distinct ids.
func seriesID(key *[9]byte, fingerprint uint64, reduced bool) []byte {
	binary.LittleEndian.PutUint64(key[:8], fingerprint)
	key[8] = 0
	if reduced {
		key[8] = 1
	}
	return key[:]
}

// setLabels builds the labels JSON and attribute maps, the expensive part of a row.
func (row *ts) setLabels(fingerprint *pkgfingerprint.Fingerprint, scopeAttrs, resourceAttrs map[string]string) {
	attrs := fingerprint.AttributesAsMap()
	row.labels = pkgfingerprint.NewLabelsAsJSONString(row.metricName, attrs, scopeAttrs, resourceAttrs)
	row.attrs = attrs
	row.scopeAttrs = scopeAttrs
	row.resourceAttrs = resourceAttrs
}

// planTimeSeries adds the registration rows one datapoint needs to the batch.
// row carries only scalar fields. Without the time bucketed set every row is
// built and the TTL cache decides at write time; with it, labels are built
// solely for rows that will be written, which in steady state is none.
func (c *clickhouseMetricsExporter) planTimeSeries(b *batch, row ts, fingerprint *pkgfingerprint.Fingerprint, scopeAttrs, resourceAttrs map[string]string, reducer *reducer, reduced *reducedSeries) {
	row.bucketStart = row.unixMilli / timeSeriesBucket.Milliseconds() * timeSeriesBucket.Milliseconds()

	if c.registry == nil {
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
		row.writeCurrent, row.writeNext = c.registry.Plan(seriesID(&key, row.fingerprint, false), row.bucketStart, b.nowMilli)
	}
	var reducedCurrent, reducedNext bool
	if reduced != nil && !b.seenTs(tsKey{fingerprint: reduced.fingerprint, reduced: true, bucketStart: row.bucketStart}) {
		reducedCurrent, reducedNext = c.registry.Plan(seriesID(&key, reduced.fingerprint, true), row.bucketStart, b.nowMilli)
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

// registeredRows yields the id and bucket of every row written from
// timeSeries, reusing one key buffer across yields.
func registeredRows(timeSeries []ts) iter.Seq2[[]byte, int64] {
	return func(yield func([]byte, int64) bool) {
		var key [9]byte
		for i := range timeSeries {
			row := &timeSeries[i]
			id := seriesID(&key, row.fingerprint, row.isReduced)
			if row.writeCurrent && !yield(id, row.bucketStart) {
				return
			}
			if row.writeNext && !yield(id, row.bucketStart+timeSeriesBucket.Milliseconds()) {
				return
			}
		}
	}
}

func (c *clickhouseMetricsExporter) appendTimeSeriesRow(statement driver.Batch, row *ts, unixMilli int64) error {
	insertedAt := time.Now().UnixMilli()
	if c.cfg.Reduction.Enabled {
		return statement.Append(
			row.env,
			row.temporality.String(),
			row.metricName,
			row.description,
			row.unit,
			row.typ.String(),
			row.isMonotonic,
			row.fingerprint,
			row.reducedFingerprint,
			row.isReduced,
			unixMilli,
			row.labels,
			row.attrs,
			row.scopeAttrs,
			row.resourceAttrs,
			false,
			insertedAt,
		)
	}
	return statement.Append(
		row.env,
		row.temporality.String(),
		row.metricName,
		row.description,
		row.unit,
		row.typ.String(),
		row.isMonotonic,
		row.fingerprint,
		unixMilli,
		row.labels,
		row.attrs,
		row.scopeAttrs,
		row.resourceAttrs,
		false,
		insertedAt,
	)
}

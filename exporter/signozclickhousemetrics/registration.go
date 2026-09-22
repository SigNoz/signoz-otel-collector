package signozclickhousemetrics

import (
	"encoding/binary"
	"iter"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

const (
	timeSeriesBucket      = time.Hour
	defaultPreWriteWindow = 15 * time.Minute
)

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

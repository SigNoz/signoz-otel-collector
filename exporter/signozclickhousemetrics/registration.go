package signozclickhousemetrics

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"go.opentelemetry.io/otel/attribute"
	metricapi "go.opentelemetry.io/otel/metric"

	pkgfingerprint "github.com/SigNoz/signoz-otel-collector/internal/common/fingerprint"
	"github.com/SigNoz/signoz-otel-collector/pkg/bucketsetcache"
)

// planTimeSeries adds the registration rows one datapoint needs to the batch.
// row carries only scalar fields; labels and attribute maps are built solely
// for rows that will be written, which in steady state is none.
func (c *clickhouseMetricsExporter) planTimeSeries(b *batch, row ts, fingerprint *pkgfingerprint.Fingerprint, scopeAttrs, resourceAttrs map[string]string, reduced *reducedSeries) {
	row.bucketStart = c.seen.BucketStart(row.unixMilli)
	var key [9]byte
	if !b.seenTs(tsKey{fingerprint: row.fingerprint, bucketStart: row.bucketStart}) {
		row.writeCur, row.writeNext = c.seen.Plan(bucketsetcache.SeriesID(&key, row.fingerprint, false), row.bucketStart, b.nowMilli)
	}

	var reducedCur, reducedNext bool
	// a series whose rule drops nothing is its own reduction: keep raw+reduced rows distinct
	if reduced != nil && !b.seenTs(tsKey{fingerprint: reduced.fingerprint, reduced: true, bucketStart: row.bucketStart}) {
		reducedCur, reducedNext = c.seen.Plan(bucketsetcache.SeriesID(&key, reduced.fingerprint, true), row.bucketStart, b.nowMilli)
	}

	if row.writeCur || row.writeNext {
		attrs := fingerprint.AttributesAsMap()
		row.labels = pkgfingerprint.NewLabelsAsJSONString(row.metricName, attrs, scopeAttrs, resourceAttrs)
		row.attrs = attrs
		row.scopeAttrs = scopeAttrs
		row.resourceAttrs = resourceAttrs
		b.addTs(&row)
	}
	if reducedCur || reducedNext {
		reducedRow := reducedTsFrom(&row, reduced)
		reducedRow.bucketStart = row.bucketStart
		reducedRow.writeCur, reducedRow.writeNext = reducedCur, reducedNext
		b.addTs(&reducedRow)
	}
}

func (c *clickhouseMetricsExporter) appendTimeSeriesRow(statement driver.Batch, ts *ts, unixMilli int64) error {
	insertedAt := time.Now().UnixMilli()
	if c.cfg.Reduction.Enabled {
		return statement.Append(
			ts.env,
			ts.temporality.String(),
			ts.metricName,
			ts.description,
			ts.unit,
			ts.typ.String(),
			ts.isMonotonic,
			ts.fingerprint,
			ts.reducedFingerprint,
			ts.isReduced,
			unixMilli,
			ts.labels,
			ts.attrs,
			ts.scopeAttrs,
			ts.resourceAttrs,
			false,
			insertedAt,
		)
	}
	return statement.Append(
		ts.env,
		ts.temporality.String(),
		ts.metricName,
		ts.description,
		ts.unit,
		ts.typ.String(),
		ts.isMonotonic,
		ts.fingerprint,
		unixMilli,
		ts.labels,
		ts.attrs,
		ts.scopeAttrs,
		ts.resourceAttrs,
		false,
		insertedAt,
	)
}

func (c *clickhouseMetricsExporter) initBucketSetCacheInstruments() error {
	var err error
	if c.registrationRows, err = c.meter.Int64Counter(
		"exporter_registration_rows",
		metricapi.WithDescription("Rows written to the time series table, by kind: cur for the datapoint's own bucket, next for pre-written next-bucket rows"),
	); err != nil {
		return err
	}
	gauges := []struct {
		dst  *metricapi.Int64Gauge
		name string
		desc string
	}{
		{&c.bucketSetEntries, "exporter_bucket_set_cache_entries", "Identities registered across live time buckets"},
		{&c.bucketSetBytes, "exporter_bucket_set_cache_bytes", "Chunk bytes allocated by the bucket set"},
		{&c.bucketSetBuckets, "exporter_bucket_set_cache_buckets", "Live time buckets"},
		{&c.bucketSetEvictions, "exporter_bucket_set_cache_evictions", "Time buckets evicted since start"},
		{&c.bucketSetCollisions, "exporter_bucket_set_cache_collisions", "Index collisions in live buckets; each one caused a duplicate row"},
		{&c.bucketSetDroppedMarks, "exporter_bucket_set_cache_dropped_marks", "Registrations dropped because their bucket was no longer live; rising means lag or clock skew beyond max_buckets"},
	}
	for _, g := range gauges {
		if *g.dst, err = c.meter.Int64Gauge(g.name, metricapi.WithDescription(g.desc)); err != nil {
			return err
		}
	}
	return nil
}

func (c *clickhouseMetricsExporter) recordRegistration(ctx context.Context, cur, next int64) {
	exporterAttr := attribute.String("exporter", c.settings.ID.String())
	c.registrationRows.Add(ctx, cur, metricapi.WithAttributes(exporterAttr, attribute.String("kind", "cur")))
	c.registrationRows.Add(ctx, next, metricapi.WithAttributes(exporterAttr, attribute.String("kind", "next")))

	stats := c.seen.Stats()
	attrs := metricapi.WithAttributes(exporterAttr)
	c.bucketSetEntries.Record(ctx, int64(stats.Entries), attrs)
	c.bucketSetBytes.Record(ctx, int64(stats.Bytes), attrs)
	c.bucketSetBuckets.Record(ctx, int64(stats.Buckets), attrs)
	c.bucketSetEvictions.Record(ctx, int64(stats.Evictions), attrs)
	c.bucketSetCollisions.Record(ctx, int64(stats.Collisions), attrs)
	c.bucketSetDroppedMarks.Record(ctx, int64(stats.DroppedMarks), attrs)
}

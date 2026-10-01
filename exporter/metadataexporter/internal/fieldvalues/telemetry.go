package fieldvalues

import (
	"context"
	"sync/atomic"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const meterScope = "github.com/SigNoz/signoz-otel-collector/exporter/metadataexporter/fieldvalues"

type telemetry struct {
	attrs               metric.MeasurementOption
	signal              attribute.KeyValue
	source              attribute.KeyValue
	rowsWritten         metric.Int64Counter
	valuesLeftOut       metric.Int64Counter
	resourcesOverflowed metric.Int64Counter
	insertErrors        metric.Int64Counter
	cacheCollisions     metric.Int64Counter
	rowsSkippedShared   metric.Int64Counter
	sharedCacheErrors   metric.Int64Counter

	// cacheUsed and cacheCapacity are read by the cache gauges.
	cacheUsed     [2]atomic.Int64
	cacheCapacity [2]atomic.Int64
}

func newTelemetry(set component.TelemetrySettings, signal, source string) (*telemetry, error) {
	meter := set.MeterProvider.Meter(meterScope)
	t := &telemetry{
		signal: attribute.String("signal", signal),
		source: attribute.String("source", source),
	}
	t.attrs = metric.WithAttributes(t.signal, t.source)
	counters := []struct {
		dst  *metric.Int64Counter
		name string
		desc string
	}{
		{&t.rowsWritten, "signoz_metadata_exporter_field_values_rows_written", "Rows written to field_values_sets."},
		{&t.valuesLeftOut, "signoz_metadata_exporter_field_values_left_out", "Pairs not written, by reason: value_length, field_places, sample_budget, cache_full."},
		{&t.resourcesOverflowed, "signoz_metadata_exporter_field_values_resources_overflowed", "Resources that started to write into their overflow set today."},
		{&t.insertErrors, "signoz_metadata_exporter_field_values_insert_errors", "Failed inserts. Their keys are not cached, so the next sighting writes them again."},
		{&t.cacheCollisions, "signoz_metadata_exporter_field_values_cache_collisions", "Keys that found no free slot in the local day cache. They are written again at their next sighting."},
		{&t.rowsSkippedShared, "signoz_metadata_exporter_field_values_rows_skipped_shared", "Rows not written because the shared cache shows that another collector wrote them today."},
		{&t.sharedCacheErrors, "signoz_metadata_exporter_field_values_shared_cache_errors", "Failed reads or writes of the shared cache. The rows are written anyway."},
	}
	for _, c := range counters {
		counter, err := meter.Int64Counter(c.name, metric.WithDescription(c.desc))
		if err != nil {
			return nil, err
		}
		*c.dst = counter
	}
	gauges := []struct {
		values *[2]atomic.Int64
		name   string
		desc   string
	}{
		{&t.cacheUsed, "signoz_metadata_exporter_field_values_cache_keys", "Keys of today in the local day cache, by part (exact or reserve)."},
		{&t.cacheCapacity, "signoz_metadata_exporter_field_values_cache_capacity", "Keys of today that each part of the local day cache can hold."},
	}
	for _, g := range gauges {
		values := g.values
		_, err := meter.Int64ObservableGauge(g.name, metric.WithDescription(g.desc),
			metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
				o.Observe(values[classExact].Load(), metric.WithAttributes(t.signal, t.source, attribute.String("part", "exact")))
				o.Observe(values[classReserve].Load(), metric.WithAttributes(t.signal, t.source, attribute.String("part", "reserve")))
				return nil
			}))
		if err != nil {
			return nil, err
		}
	}
	return t, nil
}

func (t *telemetry) recordBatch(ctx context.Context, stats batchStats) {
	for reason, n := range stats.leftOut {
		t.valuesLeftOut.Add(ctx, int64(n), metric.WithAttributes(t.signal, t.source, attribute.String("reason", string(reason))))
	}
	if stats.resourcesOverflowed > 0 {
		t.resourcesOverflowed.Add(ctx, int64(stats.resourcesOverflowed), t.attrs)
	}
}

package fieldvalues

import (
	"context"
	"sync/atomic"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const meterScope = "github.com/SigNoz/signoz-otel-collector/exporter/metadataexporter/fieldvalues"

// partAhead is the part of the cache gauges for the keys written ahead.
const partAhead = 2

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
	resourcesUntracked  metric.Int64Counter
	keysWrittenAhead    metric.Int64Counter

	// cacheUsed, cacheCapacity, trackerUsed and trackerCapacity are read by
	// the gauges.
	cacheUsed       [3]atomic.Int64
	cacheCapacity   [3]atomic.Int64
	trackerUsed     atomic.Int64
	trackerCapacity atomic.Int64
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
		{&t.valuesLeftOut, "signoz_metadata_exporter_field_values_left_out", "Pairs not written, by reason: value_length, field_places, sample_budget, cache_full, tracker_full."},
		{&t.resourcesOverflowed, "signoz_metadata_exporter_field_values_resources_overflowed", "Resources that started to write into their overflow set in the window."},
		{&t.insertErrors, "signoz_metadata_exporter_field_values_insert_errors", "Failed inserts. Their keys are not cached, so the next sighting writes them again."},
		{&t.cacheCollisions, "signoz_metadata_exporter_field_values_cache_collisions", "Keys that found no free slot in the local window cache. They are written again at their next sighting."},
		{&t.rowsSkippedShared, "signoz_metadata_exporter_field_values_rows_skipped_shared", "Rows not written because the shared cache shows that another collector wrote them in the window."},
		{&t.sharedCacheErrors, "signoz_metadata_exporter_field_values_shared_cache_errors", "Failed reads or writes of the shared cache. The rows are written anyway."},
		{&t.resourcesUntracked, "signoz_metadata_exporter_field_values_resources_untracked", "Resources of a batch without a state, because the memory of the tracker is full. Their records write into the overflow set."},
		{&t.keysWrittenAhead, "signoz_metadata_exporter_field_values_keys_written_ahead", "Keys written for the next window in the pre-write window."},
	}
	for _, c := range counters {
		counter, err := meter.Int64Counter(c.name, metric.WithDescription(c.desc))
		if err != nil {
			return nil, err
		}
		*c.dst = counter
	}
	gauges := []struct {
		values *[3]atomic.Int64
		name   string
		desc   string
	}{
		{&t.cacheUsed, "signoz_metadata_exporter_field_values_cache_keys", "Keys in the local window cache, by part: exact and reserve for the window, ahead for the next window."},
		{&t.cacheCapacity, "signoz_metadata_exporter_field_values_cache_capacity", "Keys that each part of the local window cache can hold."},
	}
	for _, g := range gauges {
		values := g.values
		_, err := meter.Int64ObservableGauge(g.name, metric.WithDescription(g.desc),
			metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
				o.Observe(values[classExact].Load(), metric.WithAttributes(t.signal, t.source, attribute.String("part", "exact")))
				o.Observe(values[classReserve].Load(), metric.WithAttributes(t.signal, t.source, attribute.String("part", "reserve")))
				o.Observe(values[partAhead].Load(), metric.WithAttributes(t.signal, t.source, attribute.String("part", "ahead")))
				return nil
			}))
		if err != nil {
			return nil, err
		}
	}
	bytes := []struct {
		value *atomic.Int64
		name  string
		desc  string
	}{
		{&t.trackerUsed, "signoz_metadata_exporter_field_values_tracker_bytes", "Memory of the value tracker and the resource states."},
		{&t.trackerCapacity, "signoz_metadata_exporter_field_values_tracker_capacity_bytes", "Memory that the value tracker and the resource states can use."},
	}
	for _, g := range bytes {
		value := g.value
		_, err := meter.Int64ObservableGauge(g.name, metric.WithDescription(g.desc), metric.WithUnit("By"),
			metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
				o.Observe(value.Load(), t.attrs)
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
	if stats.resourcesUntracked > 0 {
		t.resourcesUntracked.Add(ctx, int64(stats.resourcesUntracked), t.attrs)
	}
	if stats.keysWrittenAhead > 0 {
		t.keysWrittenAhead.Add(ctx, int64(stats.keysWrittenAhead), t.attrs)
	}
}

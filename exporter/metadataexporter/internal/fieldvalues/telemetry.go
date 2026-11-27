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
	classifierErrors    metric.Int64Counter
	classifiedAt        atomic.Int64
	rowsSkippedShared   metric.Int64Counter
	sharedCacheErrors   metric.Int64Counter
	resourcesUntracked  metric.Int64Counter
	keysWritten         metric.Int64Counter
	keyInsertErrors     metric.Int64Counter
	keysNotQueued       metric.Int64Counter
	// keysKnown and keysPending are read by the gauge.
	keysKnown   atomic.Int64
	keysPending atomic.Int64

	// cacheUsed, cacheCapacity, trackerUsed and trackerCapacity are read by
	// the gauges.
	cacheUsed       [2]atomic.Int64
	cacheCapacity   [2]atomic.Int64
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
		{&t.rowsSkippedShared, "signoz_metadata_exporter_field_values_rows_skipped_shared", "Rows not written because the shared cache shows that another collector wrote them in the window."},
		{&t.classifierErrors, "signoz_metadata_exporter_field_values_classifier_errors", "Failed reads or writes of the fields over the limit. The writer keeps the classification of the last refresh."},
		{&t.sharedCacheErrors, "signoz_metadata_exporter_field_values_shared_cache_errors", "Failed reads or writes of the shared cache. The rows are written anyway."},
		{&t.keysWritten, "signoz_metadata_exporter_field_keys_written", "Rows written to field_keys_v2. Each key is written once a day."},
		{&t.keyInsertErrors, "signoz_metadata_exporter_field_keys_insert_errors", "Failed inserts of keys. The keys wait and are inserted again."},
		{&t.keysNotQueued, "signoz_metadata_exporter_field_keys_not_queued", "Keys seen while the list of waiting keys was full. Such a key has no row until its next record."},
		{&t.resourcesUntracked, "signoz_metadata_exporter_field_values_resources_untracked", "Resources of a batch without a state, because the memory of the tracker is full. Their records write into the overflow set."},
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
		{&t.cacheUsed, "signoz_metadata_exporter_field_values_cache_keys", "Keys counted in the local key cache for the window, by part: exact and reserve."},
		{&t.cacheCapacity, "signoz_metadata_exporter_field_values_cache_capacity", "Keys that each part of the local key cache can take before the writer degrades."},
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
	_, err := meter.Int64ObservableGauge("signoz_metadata_exporter_field_keys",
		metric.WithDescription("Keys in memory, by state: written (a row of today) and waiting (no insert that passed yet)."),
		metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
			o.Observe(t.keysKnown.Load(), metric.WithAttributes(t.signal, t.source, attribute.String("state", "written")))
			o.Observe(t.keysPending.Load(), metric.WithAttributes(t.signal, t.source, attribute.String("state", "waiting")))
			return nil
		}))
	if err != nil {
		return nil, err
	}
	_, err = meter.Int64ObservableGauge("signoz_metadata_exporter_field_values_classification_time",
		metric.WithDescription("Unix time of the last read of the fields over the limit that passed, or 0 when none passed since the start. Without a read, the fields over the limit start each day inside the sets."),
		metric.WithUnit("s"),
		metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
			o.Observe(t.classifiedAt.Load(), t.attrs)
			return nil
		}))
	if err != nil {
		return nil, err
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
}

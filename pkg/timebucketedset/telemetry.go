package timebucketedset

import (
	"context"
	"errors"
	"slices"
	"sync/atomic"

	"github.com/VictoriaMetrics/fastcache"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/SigNoz/signoz-otel-collector/pkg/timebucketedset/internal/metadata"
)

type telemetry struct {
	builder *metadata.TelemetryBuilder

	miss      atomic.Int64
	hit       atomic.Int64
	preWrite  atomic.Int64
	noBucket  atomic.Int64
	applied   atomic.Int64
	ignored   atomic.Int64
	evictions atomic.Int64

	identifiers  metric.MeasurementOption
	planMiss     metric.MeasurementOption
	planHit      metric.MeasurementOption
	planPreWrite metric.MeasurementOption
	planNoBucket metric.MeasurementOption
	applyApplied metric.MeasurementOption
	applyIgnored metric.MeasurementOption
}

func newTelemetry(settings component.TelemetrySettings, identifiers []attribute.KeyValue) (*telemetry, error) {
	builder, err := metadata.NewTelemetryBuilder(settings)
	if err != nil {
		return nil, err
	}

	withResult := func(result string) metric.MeasurementOption {
		return metric.WithAttributeSet(attribute.NewSet(append(slices.Clone(identifiers), attribute.String("result", result))...))
	}

	return &telemetry{
		builder:      builder,
		identifiers:  metric.WithAttributeSet(attribute.NewSet(slices.Clone(identifiers)...)),
		planMiss:     withResult("miss"),
		planHit:      withResult("hit"),
		planPreWrite: withResult("pre_write"),
		planNoBucket: withResult("no_bucket"),
		applyApplied: withResult("applied"),
		applyIgnored: withResult("ignored"),
	}, nil
}

func (t *telemetry) register(set *Set) error {
	return errors.Join(
		t.builder.RegisterTimebucketedsetPlanIdsCallback(func(_ context.Context, observer metric.Int64Observer) error {
			observer.Observe(t.miss.Load(), t.planMiss)
			observer.Observe(t.hit.Load(), t.planHit)
			observer.Observe(t.preWrite.Load(), t.planPreWrite)
			observer.Observe(t.noBucket.Load(), t.planNoBucket)
			return nil
		}),
		t.builder.RegisterTimebucketedsetApplyIdsCallback(func(_ context.Context, observer metric.Int64Observer) error {
			observer.Observe(t.applied.Load(), t.applyApplied)
			observer.Observe(t.ignored.Load(), t.applyIgnored)
			return nil
		}),
		t.builder.RegisterTimebucketedsetBucketEvictionsCallback(func(_ context.Context, observer metric.Int64Observer) error {
			observer.Observe(t.evictions.Load(), t.identifiers)
			return nil
		}),
		t.builder.RegisterTimebucketedsetBucketCountCallback(func(_ context.Context, observer metric.Int64Observer) error {
			buckets, _ := set.stats()
			observer.Observe(int64(buckets), t.identifiers)
			return nil
		}),
		t.builder.RegisterTimebucketedsetIDCountCallback(func(_ context.Context, observer metric.Int64Observer) error {
			_, stats := set.stats()
			observer.Observe(int64(stats.EntriesCount), t.identifiers)
			return nil
		}),
		t.builder.RegisterTimebucketedsetMemoryUsageCallback(func(_ context.Context, observer metric.Int64Observer) error {
			_, stats := set.stats()
			observer.Observe(int64(stats.BytesSize), t.identifiers)
			return nil
		}),
		t.builder.RegisterTimebucketedsetMemoryLimitCallback(func(_ context.Context, observer metric.Int64Observer) error {
			_, stats := set.stats()
			observer.Observe(int64(stats.MaxBytesSize), t.identifiers)
			return nil
		}),
	)
}

func (bs *Set) stats() (int, fastcache.Stats) {
	bs.mtx.RLock()
	defer bs.mtx.RUnlock()

	var stats fastcache.Stats
	for _, bucket := range bs.buckets {
		bucket.UpdateStats(&stats)
	}

	return len(bs.buckets), stats
}

package signozlogsnormalizerprocessor

import (
	"context"
	"errors"
	"sync/atomic"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/SigNoz/signoz-otel-collector/processor/signozlogsnormalizerprocessor/internal/metadata"
)

type bodyKind int

const (
	bodyJSON bodyKind = iota
	bodyText
	bodyMap
	bodyOther
	bodyKinds
)

var bodyKindNames = [bodyKinds]string{"json", "text", "map", "other"}

type telemetry struct {
	builder *metadata.TelemetryBuilder

	records          [bodyKinds]atomic.Int64
	promotions       []atomic.Int64
	flattenings      atomic.Int64
	nestedPromotions atomic.Int64
	stringifications atomic.Int64

	recordsBody     [bodyKinds]metric.MeasurementOption
	promotionsField []metric.MeasurementOption
}

type batchStats struct {
	records                                         [bodyKinds]int64
	promotions                                      []int64
	flattenings, nestedPromotions, stringifications int64
}

func newTelemetry(settings component.TelemetrySettings, messageFields []string) (*telemetry, error) {
	builder, err := metadata.NewTelemetryBuilder(settings)
	if err != nil {
		return nil, err
	}

	t := &telemetry{
		builder:         builder,
		promotions:      make([]atomic.Int64, len(messageFields)),
		promotionsField: make([]metric.MeasurementOption, len(messageFields)),
	}
	for i, name := range bodyKindNames {
		t.recordsBody[i] = metric.WithAttributeSet(attribute.NewSet(attribute.String("body", name)))
	}
	for i, field := range messageFields {
		t.promotionsField[i] = metric.WithAttributeSet(attribute.NewSet(attribute.String("field", field)))
	}

	err = errors.Join(
		builder.RegisterSignozlogsnormalizerRecordsCallback(func(_ context.Context, observer metric.Int64Observer) error {
			for i := range t.records {
				observer.Observe(t.records[i].Load(), t.recordsBody[i])
			}
			return nil
		}),
		builder.RegisterSignozlogsnormalizerMessagePromotionsCallback(func(_ context.Context, observer metric.Int64Observer) error {
			for i := range t.promotions {
				observer.Observe(t.promotions[i].Load(), t.promotionsField[i])
			}
			return nil
		}),
		builder.RegisterSignozlogsnormalizerMessageFlatteningsCallback(func(_ context.Context, observer metric.Int64Observer) error {
			observer.Observe(t.flattenings.Load())
			return nil
		}),
		builder.RegisterSignozlogsnormalizerMessageNestedPromotionsCallback(func(_ context.Context, observer metric.Int64Observer) error {
			observer.Observe(t.nestedPromotions.Load())
			return nil
		}),
		builder.RegisterSignozlogsnormalizerMessageStringificationsCallback(func(_ context.Context, observer metric.Int64Observer) error {
			observer.Observe(t.stringifications.Load())
			return nil
		}),
	)
	if err != nil {
		builder.Shutdown()
		return nil, err
	}
	return t, nil
}

func (t *telemetry) add(st *batchStats) {
	for i, n := range st.records {
		t.records[i].Add(n)
	}
	for i, n := range st.promotions {
		t.promotions[i].Add(n)
	}
	t.flattenings.Add(st.flattenings)
	t.nestedPromotions.Add(st.nestedPromotions)
	t.stringifications.Add(st.stringifications)
}

func (t *telemetry) shutdown(context.Context) error {
	t.builder.Shutdown()
	return nil
}

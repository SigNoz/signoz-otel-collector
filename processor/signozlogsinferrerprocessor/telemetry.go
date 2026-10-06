package signozlogsinferrerprocessor

import (
	"context"
	"errors"
	"sync/atomic"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/SigNoz/signoz-otel-collector/processor/signozlogsinferrerprocessor/internal/metadata"
)

type series struct {
	count      atomic.Int64
	attributes metric.MeasurementOption
}

type telemetry struct {
	builder *metadata.TelemetryBuilder

	records    atomic.Int64
	inferences []series

	readSeries    [targetCount][][sourceDerived]int
	derivedSeries [targetCount]int
}

type batchStats struct {
	records    int64
	inferences []int64
}

func newTelemetry(settings component.TelemetrySettings, names fieldNames) (*telemetry, error) {
	builder, err := metadata.NewTelemetryBuilder(settings)
	if err != nil {
		return nil, err
	}

	t := &telemetry{builder: builder}
	var attributes []metric.MeasurementOption
	addSeries := func(tg target, field string, src source) int {
		attributes = append(attributes, metric.WithAttributeSet(attribute.NewSet(
			attribute.String("target", targetNames[tg]),
			attribute.String("field", field),
			attribute.String("source", sourceNames[src]),
		)))
		return len(attributes) - 1
	}
	for tg := range targetCount {
		t.readSeries[tg] = make([][sourceDerived]int, len(names.byTarget[tg]))
		for rank, field := range names.byTarget[tg] {
			for src := range sourceDerived {
				t.readSeries[tg][rank][src] = addSeries(tg, field, src)
			}
		}
	}
	t.derivedSeries[targetSeverityNumber] = addSeries(targetSeverityNumber, targetNames[targetSeverityText], sourceDerived)
	t.derivedSeries[targetSeverityText] = addSeries(targetSeverityText, targetNames[targetSeverityNumber], sourceDerived)

	t.inferences = make([]series, len(attributes))
	for i := range t.inferences {
		t.inferences[i].attributes = attributes[i]
	}

	err = errors.Join(
		builder.RegisterSignozlogsinferrerLogRecordsCallback(func(_ context.Context, observer metric.Int64Observer) error {
			observer.Observe(t.records.Load())
			return nil
		}),
		builder.RegisterSignozlogsinferrerFieldInferencesCallback(func(_ context.Context, observer metric.Int64Observer) error {
			for i := range t.inferences {
				if n := t.inferences[i].count.Load(); n > 0 {
					observer.Observe(n, t.inferences[i].attributes)
				}
			}
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
	t.records.Add(st.records)
	for i, n := range st.inferences {
		if n > 0 {
			t.inferences[i].count.Add(n)
		}
	}
}

func (t *telemetry) shutdown(context.Context) error {
	t.builder.Shutdown()
	return nil
}

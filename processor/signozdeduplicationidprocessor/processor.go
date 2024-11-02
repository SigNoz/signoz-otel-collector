package signozdeduplicationidprocessor

import (
	"context"
	"errors"

	"github.com/SigNoz/signoz-otel-collector/processor/signozdeduplicationidprocessor/internal/id"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
)

var (
	errTooManyDeduplicationIds = consumererror.NewPermanent(errors.New("more than one deduplication id found"))
	errMissingDepuplicationId  = consumererror.NewPermanent(errors.New("deduplication id is missing for some resources"))
)

const (
	deduplicationIdKey string = "signoz.deduplication.id"
)

type deduplicationIdProcessor struct {
	IdGenerator id.Generator
}

func newDeduplicationIdProcessor(_ processor.Settings, cfg *Config) (*deduplicationIdProcessor, error) {
	generator, err := id.GetGenerator(cfg.Generator)
	if err != nil {
		return nil, err
	}

	return &deduplicationIdProcessor{
		IdGenerator: generator,
	}, nil
}

func (processor *deduplicationIdProcessor) Start(ctx context.Context, host component.Host) error {
	return nil
}

func (processor *deduplicationIdProcessor) Shutdown(ctx context.Context) error {
	return nil
}

func (processor *deduplicationIdProcessor) ConsumeTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	rss := td.ResourceSpans()
	if rss.Len() == 0 {
		return td, nil
	}

	id, found := processor.getOrCreateId(ctx, rss.At(0).Resource())
	for i := 0; i < rss.Len(); i++ {
		val, ok := rss.At(i).Resource().Attributes().Get(deduplicationIdKey)
		if ok {
			if val.Str() != id {
				return td, errTooManyDeduplicationIds
			}
			continue
		}

		if found {
			return td, errMissingDepuplicationId
		}

		rss.At(i).Resource().Attributes().PutStr(deduplicationIdKey, id)
	}

	return td, nil
}

func (processor *deduplicationIdProcessor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	rss := md.ResourceMetrics()
	if rss.Len() == 0 {
		return md, nil
	}

	id, found := processor.getOrCreateId(ctx, rss.At(0).Resource())
	for i := 0; i < rss.Len(); i++ {
		val, ok := rss.At(i).Resource().Attributes().Get(deduplicationIdKey)
		if ok {
			if val.Str() != id {
				return md, errTooManyDeduplicationIds
			}
			continue
		}

		if found {
			return md, errMissingDepuplicationId
		}

		rss.At(i).Resource().Attributes().PutStr(deduplicationIdKey, id)
	}

	return md, nil
}

func (processor *deduplicationIdProcessor) ConsumeLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	rss := ld.ResourceLogs()
	if rss.Len() == 0 {
		return ld, nil
	}

	id, found := processor.getOrCreateId(ctx, rss.At(0).Resource())
	for i := 0; i < rss.Len(); i++ {
		val, ok := rss.At(i).Resource().Attributes().Get(deduplicationIdKey)
		if ok {
			if val.Str() != id {
				return ld, errTooManyDeduplicationIds
			}
			continue
		}

		if found {
			return ld, errMissingDepuplicationId
		}

		rss.At(i).Resource().Attributes().PutStr(deduplicationIdKey, id)
	}

	return ld, nil
}

func (processor *deduplicationIdProcessor) getOrCreateId(ctx context.Context, rs pcommon.Resource) (string, bool) {
	val, ok := rs.Attributes().Get(deduplicationIdKey)
	if ok {
		return val.Str(), true
	}

	return processor.IdGenerator.MustGenerate(ctx), false
}

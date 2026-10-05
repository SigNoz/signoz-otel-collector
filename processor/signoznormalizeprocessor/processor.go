package signoznormalizeprocessor

import (
	"context"
	"strings"

	"github.com/SigNoz/signoz-otel-collector/constants"
	"github.com/SigNoz/signoz-otel-collector/utils"
	"github.com/bytedance/sonic"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/otel/metric"
)

const (
	messageField = "message"
	scopeName    = "github.com/SigNoz/signoz-otel-collector/processor/signoznormalizeprocessor"
)

type normalizeProcessor struct {
	json          sonic.API
	stashOriginal bool
	messageFields []string
	logsProcessed metric.Int64Counter
}

func newNormalizeProcessor(set component.TelemetrySettings, cfg *Config) (*normalizeProcessor, error) {
	logsProcessed, err := set.MeterProvider.Meter(scopeName).Int64Counter(
		"signoz_normalize_processor_logs_processed",
		metric.WithDescription("Number of log records whose body was normalized by the signoznormalize processor"),
	)
	if err != nil {
		return nil, err
	}
	return &normalizeProcessor{
		json:          sonic.Config{UseInt64: true}.Froze(),
		stashOriginal: cfg.JSONBodyDualIngestion,
		messageFields: cfg.MessageFields,
		logsProcessed: logsProcessed,
	}, nil
}

func (p *normalizeProcessor) ProcessLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	var processed int64
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		sls := rls.At(i).ScopeLogs()
		for j := 0; j < sls.Len(); j++ {
			lrs := sls.At(j).LogRecords()
			for k := 0; k < lrs.Len(); k++ {
				if p.normalizeRecord(lrs.At(k)) {
					processed++
				}
			}
		}
	}
	if processed > 0 {
		p.logsProcessed.Add(ctx, processed)
	}
	return ld, nil
}

func (p *normalizeProcessor) normalizeRecord(lr plog.LogRecord) bool {
	body := lr.Body()
	if body.Type() == pcommon.ValueTypeEmpty {
		return false
	}

	original := pcommon.NewValueEmpty()
	if p.stashOriginal {
		stashOriginalBody(body, original)
	}

	switch body.Type() {
	case pcommon.ValueTypeStr:
		p.parseText(body)
	case pcommon.ValueTypeMap:
	default:
		wrapped := pcommon.NewValueMap()
		body.MoveTo(wrapped.Map().PutEmpty(messageField))
		wrapped.MoveTo(body)
	}

	p.normalizeMessage(body.Map())

	if p.stashOriginal {
		original.MoveTo(lr.Attributes().PutEmpty(constants.OriginalBodyAttributeKey))
	}
	return true
}

func stashOriginalBody(body, dest pcommon.Value) {
	if body.Type() == pcommon.ValueTypeMap {
		dest.SetStr(body.AsString())
		return
	}
	body.CopyTo(dest)
}

func (p *normalizeProcessor) parseText(body pcommon.Value) {
	str := body.Str()
	unquoted := utils.Unquote(str)
	if strings.HasPrefix(unquoted, "{") && strings.HasSuffix(unquoted, "}") {
		var parsed map[string]any
		if err := p.json.UnmarshalFromString(unquoted, &parsed); err == nil {
			if err := body.SetEmptyMap().FromRaw(parsed); err == nil {
				return
			}
		}
	}
	body.SetEmptyMap().PutStr(messageField, str)
}

func (p *normalizeProcessor) normalizeMessage(m pcommon.Map) {
	if _, ok := getMessage(m); !ok {
		for _, field := range p.messageFields {
			val, found := m.Get(field)
			if !found {
				continue
			}
			promoted := pcommon.NewValueEmpty()
			val.MoveTo(promoted)
			m.Remove(field)
			promoted.MoveTo(m.PutEmpty(messageField))
			break
		}
	}

	msg, ok := getMessage(m)
	if !ok || msg.Type() != pcommon.ValueTypeMap {
		return
	}
	inner := pcommon.NewMap()
	msg.Map().MoveTo(inner)
	m.Remove(messageField)
	inner.Range(func(k string, v pcommon.Value) bool {
		v.MoveTo(m.PutEmpty(k))
		return true
	})
}

func getMessage(m pcommon.Map) (pcommon.Value, bool) {
	val, ok := m.Get(messageField)
	if !ok {
		return pcommon.Value{}, false
	}
	if val.Type() == pcommon.ValueTypeEmpty {
		m.Remove(messageField)
		return pcommon.Value{}, false
	}
	return val, true
}

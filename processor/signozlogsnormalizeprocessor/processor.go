package signozlogsnormalizeprocessor

import (
	"context"
	"strings"

	"github.com/SigNoz/signoz-otel-collector/constants"
	"github.com/SigNoz/signoz-otel-collector/utils"
	"github.com/bytedance/sonic"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	messageField   = "message"
	jsonWhitespace = " \t\r\n"
	scopeName      = "github.com/SigNoz/signoz-otel-collector/processor/signozlogsnormalizeprocessor"
)

type normalizeProcessor struct {
	json          sonic.API
	stashOriginal bool
	messageFields []string
	telemetry     telemetry
}

type telemetry struct {
	logsProcessed          metric.Int64Counter
	logsText               metric.Int64Counter
	logsJSONParsed         metric.Int64Counter
	messagesInferred       metric.Int64Counter
	messagesFlattened      metric.Int64Counter
	messagesStringified    metric.Int64Counter
	messagesNestedPromoted metric.Int64Counter
	inferredFieldOpts      []metric.AddOption
}

type batchStats struct {
	processed, text, jsonParsed, flattened, stringified, nestedPromoted int64
	inferred                                                            []int64
}

type messageOutcome struct {
	inferredFrom   int
	flattened      bool
	stringified    bool
	nestedPromoted bool
}

func newTelemetry(set component.TelemetrySettings, messageFields []string) (telemetry, error) {
	meter := set.MeterProvider.Meter(scopeName)
	var (
		t   telemetry
		err error
	)
	counter := func(name, desc string) metric.Int64Counter {
		if err != nil {
			return nil
		}
		var c metric.Int64Counter
		c, err = meter.Int64Counter("signoz_logs_normalize_processor_"+name, metric.WithDescription(desc))
		return c
	}
	t.logsProcessed = counter("logs_processed", "Number of log records whose body was normalized by the signozlogsnormalize processor")
	t.logsText = counter("logs_text", "Number of log records whose body was plain text and became the message")
	t.logsJSONParsed = counter("logs_json_parsed", "Number of log records whose text body was parsed as a JSON object")
	t.messagesInferred = counter("messages_inferred", "Number of log records whose message was inferred from another field")
	t.messagesFlattened = counter("messages_flattened", "Number of log records whose message was an object lifted to the top level")
	t.messagesStringified = counter("messages_stringified", "Number of log records whose message is neither text nor an object and is stringified on storage by the typed body_v2.message path")
	t.messagesNestedPromoted = counter("messages_nested_promoted", "Number of log records whose lifted message object carried its own message field, which became the message")
	if err != nil {
		return telemetry{}, err
	}
	for _, f := range messageFields {
		t.inferredFieldOpts = append(t.inferredFieldOpts, metric.WithAttributes(attribute.String("field", f)))
	}
	return t, nil
}

func newNormalizeProcessor(set component.TelemetrySettings, cfg *Config) (*normalizeProcessor, error) {
	t, err := newTelemetry(set, cfg.MessageFields)
	if err != nil {
		return nil, err
	}
	return &normalizeProcessor{
		json:          sonic.Config{UseInt64: true}.Froze(),
		stashOriginal: cfg.JSONBodyDualIngestion,
		messageFields: cfg.MessageFields,
		telemetry:     t,
	}, nil
}

func (p *normalizeProcessor) ProcessLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	st := batchStats{inferred: make([]int64, len(p.messageFields))}
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		sls := rls.At(i).ScopeLogs()
		for j := 0; j < sls.Len(); j++ {
			lrs := sls.At(j).LogRecords()
			for k := 0; k < lrs.Len(); k++ {
				p.normalizeRecord(lrs.At(k), &st)
			}
		}
	}
	p.record(ctx, &st)
	return ld, nil
}

func (p *normalizeProcessor) record(ctx context.Context, st *batchStats) {
	add := func(c metric.Int64Counter, n int64) {
		if n > 0 {
			c.Add(ctx, n)
		}
	}
	add(p.telemetry.logsProcessed, st.processed)
	add(p.telemetry.logsText, st.text)
	add(p.telemetry.logsJSONParsed, st.jsonParsed)
	add(p.telemetry.messagesFlattened, st.flattened)
	add(p.telemetry.messagesStringified, st.stringified)
	add(p.telemetry.messagesNestedPromoted, st.nestedPromoted)
	for i, n := range st.inferred {
		if n > 0 {
			p.telemetry.messagesInferred.Add(ctx, n, p.telemetry.inferredFieldOpts[i])
		}
	}
}

func (p *normalizeProcessor) normalizeRecord(lr plog.LogRecord, st *batchStats) {
	body := lr.Body()
	if body.Type() == pcommon.ValueTypeEmpty {
		return
	}

	original := pcommon.NewValueEmpty()
	if p.stashOriginal {
		stashOriginalBody(body, original)
	}

	switch body.Type() {
	case pcommon.ValueTypeStr:
		if p.parseText(body) {
			st.jsonParsed++
		} else {
			st.text++
		}
	case pcommon.ValueTypeMap:
	default:
		wrapped := pcommon.NewValueMap()
		body.MoveTo(wrapped.Map().PutEmpty(messageField))
		wrapped.MoveTo(body)
	}

	out := p.normalizeMessage(body.Map())
	if out.inferredFrom >= 0 {
		st.inferred[out.inferredFrom]++
	}
	if out.flattened {
		st.flattened++
	}
	if out.stringified {
		st.stringified++
	}
	if out.nestedPromoted {
		st.nestedPromoted++
	}

	if p.stashOriginal {
		original.MoveTo(lr.Attributes().PutEmpty(constants.OriginalBodyAttributeKey))
	}
	st.processed++
}

func stashOriginalBody(body, dest pcommon.Value) {
	if body.Type() == pcommon.ValueTypeMap {
		dest.SetStr(body.AsString())
		return
	}
	body.CopyTo(dest)
}

func (p *normalizeProcessor) parseText(body pcommon.Value) bool {
	str := body.Str()
	unquoted := strings.TrimRight(utils.Unquote(strings.TrimRight(str, jsonWhitespace)), jsonWhitespace)
	if strings.HasPrefix(unquoted, "{") && strings.HasSuffix(unquoted, "}") {
		var parsed map[string]any
		if err := p.json.UnmarshalFromString(unquoted, &parsed); err == nil {
			if err := body.SetEmptyMap().FromRaw(parsed); err == nil {
				return true
			}
		}
	}
	body.SetEmptyMap().PutStr(messageField, str)
	return false
}

func (p *normalizeProcessor) normalizeMessage(m pcommon.Map) messageOutcome {
	out := messageOutcome{inferredFrom: -1}
	if _, ok := getMessage(m); !ok {
		for i, field := range p.messageFields {
			val, found := m.Get(field)
			if !found {
				continue
			}
			promoted := pcommon.NewValueEmpty()
			val.MoveTo(promoted)
			m.Remove(field)
			promoted.MoveTo(m.PutEmpty(messageField))
			out.inferredFrom = i
			break
		}
	}

	msg, ok := getMessage(m)
	if !ok {
		return out
	}
	if msg.Type() != pcommon.ValueTypeMap {
		out.stringified = msg.Type() != pcommon.ValueTypeStr
		return out
	}
	inner := pcommon.NewMap()
	msg.Map().MoveTo(inner)
	m.Remove(messageField)
	inner.Range(func(k string, v pcommon.Value) bool {
		v.MoveTo(m.PutEmpty(k))
		return true
	})
	out.flattened = true
	if lifted, ok := m.Get(messageField); ok {
		out.nestedPromoted = true
		out.stringified = lifted.Type() != pcommon.ValueTypeStr && lifted.Type() != pcommon.ValueTypeEmpty
	}
	return out
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

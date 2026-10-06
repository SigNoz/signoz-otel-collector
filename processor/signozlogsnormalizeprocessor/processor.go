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
)

const (
	messageField   = "message"
	jsonWhitespace = " \t\r\n"
)

type normalizeProcessor struct {
	json          sonic.API
	stashOriginal bool
	messageFields []string
	telemetry     *telemetry
}

type messageOutcome struct {
	inferredFrom   int
	flattened      bool
	stringified    bool
	nestedPromoted bool
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

func (p *normalizeProcessor) ProcessLogs(_ context.Context, ld plog.Logs) (plog.Logs, error) {
	st := batchStats{promotions: make([]int64, len(p.messageFields))}
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
	p.telemetry.add(&st)
	return ld, nil
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

	kind := bodyOther
	switch body.Type() {
	case pcommon.ValueTypeStr:
		kind = bodyText
		if p.parseText(body) {
			kind = bodyJSON
		}
	case pcommon.ValueTypeMap:
		kind = bodyMap
	default:
		wrapped := pcommon.NewValueMap()
		body.MoveTo(wrapped.Map().PutEmpty(messageField))
		wrapped.MoveTo(body)
	}

	out := p.normalizeMessage(body.Map())
	if out.inferredFrom >= 0 {
		st.promotions[out.inferredFrom]++
	}
	if out.flattened {
		st.flattenings++
	}
	if out.stringified {
		st.stringifications++
	}
	if out.nestedPromoted {
		st.nestedPromotions++
	}

	if p.stashOriginal {
		original.MoveTo(lr.Attributes().PutEmpty(constants.OriginalBodyAttributeKey))
	}
	st.records[kind]++
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
	unquoted := strings.Trim(utils.Unquote(strings.Trim(str, jsonWhitespace)), jsonWhitespace)
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
			if val.Type() == pcommon.ValueTypeEmpty {
				m.Remove(field)
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

package signozlogsnormalizerprocessor

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
	bodyEnabled   bool
	messageFields []string
	names         fieldNames
	telemetry     *telemetry
}

type messageOutcome struct {
	inferredFrom   int
	flattened      bool
	stringified    bool
	nestedPromoted bool
}

func newNormalizeProcessor(set component.TelemetrySettings, cfg *Config) (*normalizeProcessor, error) {
	names := newFieldNames(cfg.Fields)
	t, err := newTelemetry(set, cfg.Body.MessageFields, &names)
	if err != nil {
		return nil, err
	}
	return &normalizeProcessor{
		json:          sonic.Config{UseInt64: true}.Froze(),
		bodyEnabled:   cfg.Body.Enabled,
		messageFields: cfg.Body.MessageFields,
		names:         names,
		telemetry:     t,
	}, nil
}

func (p *normalizeProcessor) ProcessLogs(_ context.Context, ld plog.Logs) (plog.Logs, error) {
	if !p.bodyEnabled && !p.names.any() {
		return ld, nil
	}
	st := batchStats{
		promotions: make([]int64, len(p.messageFields)),
		inferences: make([]int64, len(p.telemetry.inferences)),
	}
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		p.processResource(rls.At(i), &st)
	}
	p.telemetry.add(&st)
	return ld, nil
}

func (p *normalizeProcessor) normalizeBody(lr plog.LogRecord, st *batchStats) {
	lr.Attributes().Remove(constants.OriginalBodyAttributeKey)
	body := lr.Body()
	if body.Type() == pcommon.ValueTypeEmpty {
		return
	}

	original := pcommon.NewValueEmpty()
	st.scratch = stashOriginalBody(body, original, st.scratch)

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

	original.MoveTo(lr.Attributes().PutEmpty(constants.OriginalBodyAttributeKey))
	st.records[kind]++
}

func stashOriginalBody(body, dest pcommon.Value, scratch []byte) []byte {
	if body.Type() != pcommon.ValueTypeMap {
		body.CopyTo(dest)
		return scratch
	}
	encoded, ok := appendAsString(scratch[:0], body)
	if ok {
		dest.SetStr(string(encoded))
	} else {
		dest.SetStr("")
	}
	return encoded
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

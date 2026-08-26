package signozllmpricingprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"

	"github.com/SigNoz/signoz-otel-collector/processor/signozllmpricingprocessor/internal/metadatatest"
)

// baseTelemetryAttrs mirrors the attribute set the processor stamps on every
// measurement, so assertions only have to add the varying `reason`.
func baseTelemetryAttrs(t *testing.T, tt *componenttest.Telemetry) []attribute.KeyValue {
	t.Helper()
	return []attribute.KeyValue{
		attribute.String("processor", metadatatest.NewSettings(tt).ID.String()),
		attribute.String("otel.signal", "traces"),
	}
}

// runWithTelemetry processes one span built from spanAttrs and returns the
// telemetry recorder for assertions.
func runWithTelemetry(t *testing.T, cfg *Config, spanAttrs map[string]any) *componenttest.Telemetry {
	t.Helper()
	tt := componenttest.NewTelemetry()
	t.Cleanup(func() { require.NoError(t, tt.Shutdown(context.Background())) })

	p, err := newProcessor(cfg, metadatatest.NewSettings(tt))
	require.NoError(t, err)

	_, err = p.ProcessTraces(context.Background(), buildTrace(spanAttrs))
	require.NoError(t, err)
	return tt
}

func assertUnpricedReason(t *testing.T, tt *componenttest.Telemetry, reason string) {
	t.Helper()
	metadatatest.AssertEqualProcessorSignozllmpricingUnpricedSpans(t, tt,
		[]metricdata.DataPoint[int64]{{
			Value:      1,
			Attributes: attribute.NewSet(append(baseTelemetryAttrs(t, tt), attribute.String("reason", reason))...),
		}},
		metricdatatest.IgnoreTimestamp(),
	)

	// A skipped span must never be counted as priced.
	_, err := tt.GetMetric("otelcol_processor_signozllmpricing_priced_spans")
	require.Error(t, err, "priced_spans must not be recorded for a skipped span")
}

func TestTelemetry_PricedSpan(t *testing.T) {
	tt := runWithTelemetry(t, testCfg, map[string]any{
		"gen_ai.request.model":       "gpt-4o",
		"gen_ai.usage.input_tokens":  int64(1000),
		"gen_ai.usage.output_tokens": int64(500),
	})

	metadatatest.AssertEqualProcessorSignozllmpricingPricedSpans(t, tt,
		[]metricdata.DataPoint[int64]{{
			Value:      1,
			Attributes: attribute.NewSet(baseTelemetryAttrs(t, tt)...),
		}},
		metricdatatest.IgnoreTimestamp(),
	)

	_, err := tt.GetMetric("otelcol_processor_signozllmpricing_unpriced_spans")
	require.Error(t, err, "unpriced_spans must not be recorded for a priced span")
}

func TestTelemetry_MissingModelAttr(t *testing.T) {
	tt := runWithTelemetry(t, testCfg, map[string]any{
		"gen_ai.usage.input_tokens": int64(1000),
	})
	assertUnpricedReason(t, tt, "missing_model_attr")
}

func TestTelemetry_NoRuleMatch(t *testing.T) {
	tt := runWithTelemetry(t, testCfg, map[string]any{
		"gen_ai.request.model":      "unknown-model-xyz",
		"gen_ai.usage.input_tokens": int64(1000),
	})
	assertUnpricedReason(t, tt, "no_rule_match")
}

func TestTelemetry_ZeroTokens(t *testing.T) {
	tt := runWithTelemetry(t, testCfg, map[string]any{
		"gen_ai.request.model": "gpt-4o",
	})
	assertUnpricedReason(t, tt, "zero_tokens")
}

// A token count arriving as a string is reported distinctly from a genuine zero
// — this is the case that was previously indistinguishable from "no usage".
func TestTelemetry_NonNumericTokenValue(t *testing.T) {
	tt := runWithTelemetry(t, testCfg, map[string]any{
		"gen_ai.request.model":      "gpt-4o",
		"gen_ai.usage.input_tokens": "1000",
	})
	assertUnpricedReason(t, tt, "non_numeric_token_value")
}

// A non-numeric value alongside a usable count still prices the span; the
// malformed value must not suppress pricing.
func TestTelemetry_NonNumericTokenValue_StillPricesWhenOtherCountsUsable(t *testing.T) {
	tt := runWithTelemetry(t, testCfg, map[string]any{
		"gen_ai.request.model":       "gpt-4o",
		"gen_ai.usage.input_tokens":  "not-a-number",
		"gen_ai.usage.output_tokens": int64(500),
	})

	metadatatest.AssertEqualProcessorSignozllmpricingPricedSpans(t, tt,
		[]metricdata.DataPoint[int64]{{
			Value:      1,
			Attributes: attribute.NewSet(baseTelemetryAttrs(t, tt)...),
		}},
		metricdatatest.IgnoreTimestamp(),
	)
}

// Every span in a multi-span batch must be accounted for exactly once, so
// priced + unpriced together reconcile against total spans seen.
func TestTelemetry_CountsEverySpanInBatch(t *testing.T) {
	tt := componenttest.NewTelemetry()
	t.Cleanup(func() { require.NoError(t, tt.Shutdown(context.Background())) })

	p, err := newProcessor(testCfg, metadatatest.NewSettings(tt))
	require.NoError(t, err)

	td := buildTrace(map[string]any{
		"gen_ai.request.model":       "gpt-4o",
		"gen_ai.usage.input_tokens":  int64(1000),
		"gen_ai.usage.output_tokens": int64(500),
	})
	// Two more spans in the same scope: one unmatched model, one with no model.
	spans := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans()
	unmatched := spans.AppendEmpty()
	unmatched.Attributes().PutStr("gen_ai.request.model", "unknown-model-xyz")
	unmatched.Attributes().PutInt("gen_ai.usage.input_tokens", 10)
	spans.AppendEmpty() // no attributes at all

	_, err = p.ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	base := baseTelemetryAttrs(t, tt)
	metadatatest.AssertEqualProcessorSignozllmpricingPricedSpans(t, tt,
		[]metricdata.DataPoint[int64]{{
			Value:      1,
			Attributes: attribute.NewSet(base...),
		}},
		metricdatatest.IgnoreTimestamp(),
	)
	metadatatest.AssertEqualProcessorSignozllmpricingUnpricedSpans(t, tt,
		[]metricdata.DataPoint[int64]{
			{
				Value:      1,
				Attributes: attribute.NewSet(append(append([]attribute.KeyValue{}, base...), attribute.String("reason", "missing_model_attr"))...),
			},
			{
				Value:      1,
				Attributes: attribute.NewSet(append(append([]attribute.KeyValue{}, base...), attribute.String("reason", "no_rule_match"))...),
			},
		},
		metricdatatest.IgnoreTimestamp(),
	)
}

package signozspanmappingprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// buildTrace returns a trace with a single span. The caller receives pointers
// to the span attributes and resource attributes for setup.
func buildTrace(t *testing.T, spanAttrs map[string]string, resAttrs map[string]string) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	for k, v := range resAttrs {
		rs.Resource().Attributes().PutStr(k, v)
	}
	span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	for k, v := range spanAttrs {
		span.Attributes().PutStr(k, v)
	}
	return td
}

func spanAttrs(t *testing.T, td ptrace.Traces) pcommon.Map {
	t.Helper()
	return td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes()
}

func resAttrs(t *testing.T, td ptrace.Traces) pcommon.Map {
	t.Helper()
	return td.ResourceSpans().At(0).Resource().Attributes()
}

func TestGlobMatchInSpanAttrs(t *testing.T) {
	cfg := &Config{
		Groups: []Group{
			{
				ID:        "llm",
				ExistsAny: ExistsAny{Attributes: []string{"model"}},
				Attributes: []AttributeRule{
					{Target: "gen_ai.request.model", Sources: []Source{{Key: "llm.model"}}},
				},
			},
		},
	}
	require.NoError(t, cfg.Validate())

	td := buildTrace(
		t,
		map[string]string{"llm.model": "gpt-4", "gen_ai.llm.model": "gpt-40"},
		nil,
	)
	_, err := newProcessor(cfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	val, ok := spanAttrs(t, td).Get("gen_ai.request.model")
	require.True(t, ok)
	assert.Equal(t, "gpt-4", val.Str())
}

func TestGlobMatchInResourceAttrs(t *testing.T) {
	cfg := &Config{
		Groups: []Group{
			{
				ID:        "llm",
				ExistsAny: ExistsAny{Resource: []string{"service.name"}},
				Attributes: []AttributeRule{
					{
						Target:  "gen_ai.request.model",
						Sources: []Source{{Key: "resource.service.name"}},
					},
				},
			},
		},
	}
	require.NoError(t, cfg.Validate())

	td := buildTrace(t, nil, map[string]string{"service.name": "my-llm-service"})
	_, err := newProcessor(cfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	val, ok := spanAttrs(t, td).Get("gen_ai.request.model")
	require.True(t, ok)
	assert.Equal(t, "my-llm-service", val.Str())
}

func TestNoMatchSkipsGroup(t *testing.T) {
	cfg := &Config{
		Groups: []Group{
			{
				ID:        "llm",
				ExistsAny: ExistsAny{Attributes: []string{"model"}},
				Attributes: []AttributeRule{
					{Target: "gen_ai.request.model", Sources: []Source{{Key: "llm.model"}}},
				},
			},
		},
	}
	require.NoError(t, cfg.Validate())

	// span has no attribute containing "model"
	td := buildTrace(t, map[string]string{"some.other.key": "value"}, nil)
	_, err := newProcessor(cfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	_, ok := spanAttrs(t, td).Get("gen_ai.request.model")
	assert.False(t, ok, "target must not be set when condition is not met")
}

func TestSourceFirstMatchWins(t *testing.T) {
	cfg := &Config{
		Groups: []Group{
			{
				ID:        "tokens",
				ExistsAny: ExistsAny{Attributes: []string{"llm"}},
				Attributes: []AttributeRule{
					{
						Target: "gen_ai.request.tokens",
						Sources: []Source{
							{Key: "gen_ai.request_tokens"},
							{Key: "llm.tokens"},
						},
					},
				},
			},
		},
	}
	require.NoError(t, cfg.Validate())

	// Both keys present — gen_ai.request_tokens wins.
	td := buildTrace(
		t,
		map[string]string{
			"gen_ai.request_tokens": "100",
			"llm.tokens":            "200",
		},
		nil,
	)
	_, err := newProcessor(cfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	val, ok := spanAttrs(t, td).Get("gen_ai.request.tokens")
	require.True(t, ok)
	assert.Equal(t, "100", val.Str())
}

func TestSourceFallsBackToSecond(t *testing.T) {
	cfg := &Config{
		Groups: []Group{
			{
				ID:        "tokens",
				ExistsAny: ExistsAny{Attributes: []string{"llm"}},
				Attributes: []AttributeRule{
					{
						Target: "gen_ai.request.tokens",
						Sources: []Source{
							{Key: "gen_ai.request_tokens"},
							{Key: "llm.tokens"},
						},
					},
				},
			},
		},
	}
	require.NoError(t, cfg.Validate())

	// Only llm.tokens present.
	td := buildTrace(t, map[string]string{"llm.tokens": "200"}, nil)
	_, err := newProcessor(cfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	val, ok := spanAttrs(t, td).Get("gen_ai.request.tokens")
	require.True(t, ok)
	assert.Equal(t, "200", val.Str())
}

func TestActionMove(t *testing.T) {
	cfg := &Config{
		Groups: []Group{
			{
				ID:        "input",
				ExistsAny: ExistsAny{Attributes: []string{"gen_ai"}},
				Attributes: []AttributeRule{
					{
						Target:  "gen_ai.request.input",
						Sources: []Source{{Key: "gen_ai.input", Action: ActionMove}},
					},
				},
			},
		},
	}
	require.NoError(t, cfg.Validate())

	td := buildTrace(t, map[string]string{"gen_ai.input": "hello"}, nil)
	_, err := newProcessor(cfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	val, ok := spanAttrs(t, td).Get("gen_ai.request.input")
	require.True(t, ok)
	assert.Equal(t, "hello", val.Str())

	// Source must have been deleted.
	_, srcPresent := spanAttrs(t, td).Get("gen_ai.input")
	assert.False(t, srcPresent, "source key must be deleted when action=move")
}

func TestActionCopy(t *testing.T) {
	cfg := &Config{
		Groups: []Group{
			{
				ID:        "input",
				ExistsAny: ExistsAny{Attributes: []string{"gen_ai"}},
				Attributes: []AttributeRule{
					{
						Target:  "gen_ai.request.input",
						Sources: []Source{{Key: "gen_ai.input", Action: ActionCopy}},
					},
				},
			},
		},
	}
	require.NoError(t, cfg.Validate())

	td := buildTrace(t, map[string]string{"gen_ai.input": "hello"}, nil)
	_, err := newProcessor(cfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	// Source must still be present.
	_, srcPresent := spanAttrs(t, td).Get("gen_ai.input")
	assert.True(t, srcPresent, "source key must be kept when action=copy")
}

// TestPerSourceAction asserts that move and copy on sibling sources of the
// same rule are each honored independently — the central guarantee of the
// per-source action design.
func TestPerSourceAction(t *testing.T) {
	cfg := &Config{
		Groups: []Group{
			{
				ID:        "mixed",
				ExistsAny: ExistsAny{Attributes: []string{"input"}},
				Attributes: []AttributeRule{
					{
						Target: "gen_ai.request.input",
						Sources: []Source{
							{Key: "gen_ai.input", Action: ActionMove},
							{Key: "llm.input", Action: ActionCopy},
						},
					},
				},
			},
		},
	}
	require.NoError(t, cfg.Validate())

	// First source matches → must be moved (deleted).
	td := buildTrace(t, map[string]string{"gen_ai.input": "first", "llm.input": "second"}, nil)
	_, err := newProcessor(cfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	val, ok := spanAttrs(t, td).Get("gen_ai.request.input")
	require.True(t, ok)
	assert.Equal(t, "first", val.Str())
	_, gone := spanAttrs(t, td).Get("gen_ai.input")
	assert.False(t, gone, "gen_ai.input must be removed when its action=move and it was the matching source")
	_, kept := spanAttrs(t, td).Get("llm.input")
	assert.True(t, kept, "llm.input must be untouched — it was never the matching source")

	// Only the second source is present → must be copied (kept).
	td = buildTrace(t, map[string]string{"llm.input": "only"}, nil)
	_, err = newProcessor(cfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	val, ok = spanAttrs(t, td).Get("gen_ai.request.input")
	require.True(t, ok)
	assert.Equal(t, "only", val.Str())
	_, kept = spanAttrs(t, td).Get("llm.input")
	assert.True(t, kept, "llm.input must be kept when its action=copy")
}

func TestContextResource(t *testing.T) {
	cfg := &Config{
		Groups: []Group{
			{
				ID:        "llm",
				ExistsAny: ExistsAny{Attributes: []string{"llm"}},
				Attributes: []AttributeRule{
					{
						Target:  "gen_ai.request.model",
						Context: ContextResource,
						Sources: []Source{{Key: "llm.model"}},
					},
				},
			},
		},
	}
	require.NoError(t, cfg.Validate())

	td := buildTrace(t, map[string]string{"llm.model": "gpt-4o"}, nil)
	_, err := newProcessor(cfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	// Must appear in resource attributes.
	val, ok := resAttrs(t, td).Get("gen_ai.request.model")
	require.True(t, ok, "target must be in resource attrs when context=resource")
	assert.Equal(t, "gpt-4o", val.Str())

	// Must NOT appear in span attributes.
	_, inSpan := spanAttrs(t, td).Get("gen_ai.request.model")
	assert.False(t, inSpan)
}

func TestLLMGroupScenario(t *testing.T) {
	cfg := &Config{
		Groups: []Group{
			{
				ID: "llm",
				ExistsAny: ExistsAny{
					Attributes: []string{"mode"},
					Resource:   []string{"service.name"},
				},
				Attributes: []AttributeRule{
					{
						Target:  "gen_ai.request.model",
						Context: ContextResource,
						Sources: []Source{
							{Key: "gen_ai.llm.model"},
							{Key: "llm.model"},
							{Key: "resource.service.name"},
						},
					},
					{
						Target: "gen_ai.request.tokens",
						Sources: []Source{
							{Key: "gen_ai.request_tokens"},
							{Key: "llm.tokens"},
						},
					},
					{
						Target: "gen_ai.request.input",
						Sources: []Source{
							{Key: "gen_ai.input", Action: ActionMove},
							{Key: "llm.input", Action: ActionMove},
						},
					},
				},
			},
		},
	}
	require.NoError(t, cfg.Validate())

	td := buildTrace(
		t,
		map[string]string{
			"llm.model":    "gpt-4",
			"llm.tokens":   "512",
			"gen_ai.input": "tell me a story",
		},
		map[string]string{"service.name": "my-llm-service"},
	)

	_, err := newProcessor(cfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	// gen_ai.request.model written to resource (context=resource), source=llm.model wins
	modelVal, ok := resAttrs(t, td).Get("gen_ai.request.model")
	require.True(t, ok)
	assert.Equal(t, "gpt-4", modelVal.Str())

	// gen_ai.request.tokens from llm.tokens (fallback, gen_ai.request_tokens absent)
	tokVal, ok := spanAttrs(t, td).Get("gen_ai.request.tokens")
	require.True(t, ok)
	assert.Equal(t, "512", tokVal.Str())

	// gen_ai.request.input moved from gen_ai.input
	inputVal, ok := spanAttrs(t, td).Get("gen_ai.request.input")
	require.True(t, ok)
	assert.Equal(t, "tell me a story", inputVal.Str())
	_, srcPresent := spanAttrs(t, td).Get("gen_ai.input")
	assert.False(t, srcPresent)
}

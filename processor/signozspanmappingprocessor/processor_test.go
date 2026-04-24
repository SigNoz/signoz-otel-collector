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
func buildTrace(
	spanAttrs map[string]string,
	resAttrs map[string]string,
) ptrace.Traces {
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

func spanAttrs(td ptrace.Traces) pcommon.Map {
	return td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes()
}

func resAttrs(td ptrace.Traces) pcommon.Map {
	return td.ResourceSpans().At(0).Resource().Attributes()
}

func TestGlobMatchInSpanAttrs(t *testing.T) {
	cfg := &Config{
		Groups: []Group{
			{
				ID:        "llm",
				ExistsAny: ExistsAny{Attributes: []string{"*model*"}},
				Attributes: []AttributeRule{
					{Target: "gen_ai.request.model", Sources: []string{"llm.model"}},
				},
			},
		},
	}
	require.NoError(t, cfg.Validate())

	td := buildTrace(
		map[string]string{"llm.model": "gpt-4", "gen_ai.llm.model": "gpt-40"},
		nil,
	)
	_, err := newProcessor(cfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	val, ok := spanAttrs(td).Get("gen_ai.request.model")
	require.True(t, ok)
	assert.Equal(t, "gpt-4", val.Str())
}

func TestGlobMatchInResourceAttrs(t *testing.T) {
	cfg := &Config{
		Groups: []Group{
			{
				ID:        "llm",
				ExistsAny: ExistsAny{Resource: []string{"service.name*"}},
				Attributes: []AttributeRule{
					{
						Target:  "gen_ai.request.model",
						Sources: []string{"resource.service.name"},
					},
				},
			},
		},
	}
	require.NoError(t, cfg.Validate())

	td := buildTrace(nil, map[string]string{"service.name": "my-llm-service"})
	_, err := newProcessor(cfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	val, ok := spanAttrs(td).Get("gen_ai.request.model")
	require.True(t, ok)
	assert.Equal(t, "my-llm-service", val.Str())
}

func TestNoMatchSkipsGroup(t *testing.T) {
	cfg := &Config{
		Groups: []Group{
			{
				ID:        "llm",
				ExistsAny: ExistsAny{Attributes: []string{"*model*"}},
				Attributes: []AttributeRule{
					{Target: "gen_ai.request.model", Sources: []string{"llm.model"}},
				},
			},
		},
	}
	require.NoError(t, cfg.Validate())

	// span has no attribute containing "model"
	td := buildTrace(map[string]string{"some.other.key": "value"}, nil)
	_, err := newProcessor(cfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	_, ok := spanAttrs(td).Get("gen_ai.request.model")
	assert.False(t, ok, "target must not be set when condition is not met")
}

func TestSourceFirstMatchWins(t *testing.T) {
	cfg := &Config{
		Groups: []Group{
			{
				ID:        "tokens",
				ExistsAny: ExistsAny{Attributes: []string{"llm.*"}},
				Attributes: []AttributeRule{
					{
						Target:  "gen_ai.request.tokens",
						Sources: []string{"gen_ai.request_tokens", "llm.tokens"},
					},
				},
			},
		},
	}
	require.NoError(t, cfg.Validate())

	// Both keys present — gen_ai.request_tokens wins.
	td := buildTrace(
		map[string]string{
			"gen_ai.request_tokens": "100",
			"llm.tokens":            "200",
		},
		nil,
	)
	_, err := newProcessor(cfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	val, ok := spanAttrs(td).Get("gen_ai.request.tokens")
	require.True(t, ok)
	assert.Equal(t, "100", val.Str())
}

func TestSourceFallsBackToSecond(t *testing.T) {
	cfg := &Config{
		Groups: []Group{
			{
				ID:        "tokens",
				ExistsAny: ExistsAny{Attributes: []string{"llm.*"}},
				Attributes: []AttributeRule{
					{
						Target:  "gen_ai.request.tokens",
						Sources: []string{"gen_ai.request_tokens", "llm.tokens"},
					},
				},
			},
		},
	}
	require.NoError(t, cfg.Validate())

	// Only llm.tokens present.
	td := buildTrace(map[string]string{"llm.tokens": "200"}, nil)
	_, err := newProcessor(cfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	val, ok := spanAttrs(td).Get("gen_ai.request.tokens")
	require.True(t, ok)
	assert.Equal(t, "200", val.Str())
}

func TestActionMove(t *testing.T) {
	cfg := &Config{
		Groups: []Group{
			{
				ID:        "input",
				ExistsAny: ExistsAny{Attributes: []string{"gen_ai.*"}},
				Attributes: []AttributeRule{
					{
						Target:  "gen_ai.request.input",
						Sources: []string{"gen_ai.input"},
						Action:  ActionMove,
					},
				},
			},
		},
	}
	require.NoError(t, cfg.Validate())

	td := buildTrace(map[string]string{"gen_ai.input": "hello"}, nil)
	_, err := newProcessor(cfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	val, ok := spanAttrs(td).Get("gen_ai.request.input")
	require.True(t, ok)
	assert.Equal(t, "hello", val.Str())

	// Source must have been deleted.
	_, srcPresent := spanAttrs(td).Get("gen_ai.input")
	assert.False(t, srcPresent, "source key must be deleted when action=move")
}

func TestActionCopy(t *testing.T) {
	cfg := &Config{
		Groups: []Group{
			{
				ID:        "input",
				ExistsAny: ExistsAny{Attributes: []string{"gen_ai.*"}},
				Attributes: []AttributeRule{
					{
						Target:  "gen_ai.request.input",
						Sources: []string{"gen_ai.input"},
						Action:  ActionCopy,
					},
				},
			},
		},
	}
	require.NoError(t, cfg.Validate())

	td := buildTrace(map[string]string{"gen_ai.input": "hello"}, nil)
	_, err := newProcessor(cfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	// Source must still be present.
	_, srcPresent := spanAttrs(td).Get("gen_ai.input")
	assert.True(t, srcPresent, "source key must be kept when action=copy")
}

func TestContextResource(t *testing.T) {
	cfg := &Config{
		Groups: []Group{
			{
				ID:        "llm",
				ExistsAny: ExistsAny{Attributes: []string{"llm.*"}},
				Attributes: []AttributeRule{
					{
						Target:  "gen_ai.request.model",
						Context: ContextResource,
						Sources: []string{"llm.model"},
					},
				},
			},
		},
	}
	require.NoError(t, cfg.Validate())

	td := buildTrace(map[string]string{"llm.model": "gpt-4o"}, nil)
	_, err := newProcessor(cfg).ProcessTraces(context.Background(), td)
	require.NoError(t, err)

	// Must appear in resource attributes.
	val, ok := resAttrs(td).Get("gen_ai.request.model")
	require.True(t, ok, "target must be in resource attrs when context=resource")
	assert.Equal(t, "gpt-4o", val.Str())

	// Must NOT appear in span attributes.
	_, inSpan := spanAttrs(td).Get("gen_ai.request.model")
	assert.False(t, inSpan)
}

func TestLLMGroupScenario(t *testing.T) {
	cfg := &Config{
		Groups: []Group{
			{
				ID: "llm",
				ExistsAny: ExistsAny{
					Attributes: []string{"*model*"},
					Resource:   []string{"service.name*"},
				},
				Attributes: []AttributeRule{
					{
						Target:  "gen_ai.request.model",
						Context: ContextResource,
						Sources: []string{"gen_ai.llm.model", "llm.model", "resource.service.name"},
					},
					{
						Target:  "gen_ai.request.tokens",
						Sources: []string{"gen_ai.request_tokens", "llm.tokens"},
					},
					{
						Target:  "gen_ai.request.input",
						Action:  ActionMove,
						Sources: []string{"gen_ai.input", "llm.input"},
					},
				},
			},
		},
	}
	require.NoError(t, cfg.Validate())

	td := buildTrace(
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
	modelVal, ok := resAttrs(td).Get("gen_ai.request.model")
	require.True(t, ok)
	assert.Equal(t, "gpt-4", modelVal.Str())

	// gen_ai.request.tokens from llm.tokens (fallback, gen_ai.request_tokens absent)
	tokVal, ok := spanAttrs(td).Get("gen_ai.request.tokens")
	require.True(t, ok)
	assert.Equal(t, "512", tokVal.Str())

	// gen_ai.request.input moved from gen_ai.input
	inputVal, ok := spanAttrs(td).Get("gen_ai.request.input")
	require.True(t, ok)
	assert.Equal(t, "tell me a story", inputVal.Str())
	_, srcPresent := spanAttrs(td).Get("gen_ai.input")
	assert.False(t, srcPresent)
}

package signozspanmapperprocessor

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"
)

// 50 resource spans × 200 spans each = 10,000 spans per batch.
const (
	benchResourceSpans = 50
	benchSpansPerScope = 200
)

// processor config
func threeGroupConfig() *Config {
	return &Config{
		Groups: []Group{
			{
				ID:        "model",
				ExistsAny: ExistsAny{Attributes: []string{"model"}},
				Attributes: []AttributeRule{
					{
						Target:  "gen_ai.request.model",
						Context: ContextResource,
						Sources: []Source{
							{Key: "gen_ai.model"},
							{Key: "llm.model"},
						},
					},
					{
						Target: "gen_ai.request.tokens",
						Sources: []Source{
							{Key: "gen_ai.request_tokens"},
							{Key: "llm.tokens"},
						},
					},
				},
			},
			{
				ID:        "tool",
				ExistsAny: ExistsAny{Attributes: []string{"tool"}},
				Attributes: []AttributeRule{
					{
						Target: "gen_ai.tool.name",
						Sources: []Source{
							{Key: "gen_ai.tool.name"},
							{Key: "llm.tool.name"},
						},
					},
					{
						Target: "gen_ai.tool.arguments",
						Sources: []Source{
							{Key: "tool.args", Action: ActionMove},
						},
					},
				},
			},
			{
				ID:        "agent",
				ExistsAny: ExistsAny{Attributes: []string{"agent"}},
				Attributes: []AttributeRule{
					{
						Target: "gen_ai.agent.name",
						Sources: []Source{
							{Key: "gen_ai.agent.name"},
							{Key: "llm.agent.name", Action: ActionMove},
						},
					},
				},
			},
		},
	}
}

// commonAttrs is non-matching noise present on every span, so the exists_any
// substring scan has realistic keys to walk past before it finds (or fails to
// find) a match.
func commonAttrs(a pcommon.Map) {
	a.PutStr("http.method", "POST")
	a.PutStr("http.status_code", "200")
	a.PutStr("net.peer.name", "api.example.com")
}

func modelAttrs(a pcommon.Map) {
	a.PutStr("llm.model", "gpt-4")
	a.PutStr("llm.tokens", "512")
}
func toolAttrs(a pcommon.Map) {
	a.PutStr("llm.tool.name", "web_search")
	a.PutStr("tool.args", `{"q":"otel"}`)
}
func agentAttrs(a pcommon.Map) {
	a.PutStr("llm.agent.name", "planner")
}

// variation describes how the spans in a batch are populated, letting each
// benchmark exercise a different match pattern against the three groups.
type variation struct {
	name string
	fill func(a pcommon.Map, spanIdx int)
}

var variations = []variation{
	// Every span matches exactly one group.
	{"model_only", func(a pcommon.Map, _ int) { modelAttrs(a) }},
	{"tool_only", func(a pcommon.Map, _ int) { toolAttrs(a) }},
	{"agent_only", func(a pcommon.Map, _ int) { agentAttrs(a) }},
	// Spans rotate across the three groups (one group matches each span).
	{"mixed", func(a pcommon.Map, i int) {
		switch i % 3 {
		case 0:
			modelAttrs(a)
		case 1:
			toolAttrs(a)
		default:
			agentAttrs(a)
		}
	}},
	// No span matches any group — worst case for the exists_any scan: all
	// three groups' patterns are checked against every key and never match.
	{"no_match", func(pcommon.Map, int) {}},
}

// newBenchTraces builds a fresh batch every call. A fresh batch is required per
// iteration because the processor mutates the data in place (move actions
// delete source attributes), so reusing a batch would not reflect steady-state
// work.
func newBenchTraces(v variation) ptrace.Traces {
	td := ptrace.NewTraces()
	for r := range benchResourceSpans {
		rs := td.ResourceSpans().AppendEmpty()
		rs.Resource().Attributes().PutStr("service.name", fmt.Sprintf("llm-service-%d", r))
		rs.Resource().Attributes().PutStr("host.name", fmt.Sprintf("node-%d", r))
		ss := rs.ScopeSpans().AppendEmpty()
		for s := range benchSpansPerScope {
			span := ss.Spans().AppendEmpty()
			span.SetName(fmt.Sprintf("span-%d", s))
			a := span.Attributes()
			commonAttrs(a)
			v.fill(a, s)
		}
	}
	return td
}

// BenchmarkReceiverExporter is the baseline pipeline: a receiver feeding straight
func BenchmarkReceiverExporter(b *testing.B) {
	exporter := consumertest.NewNop()
	ctx := context.Background()

	for _, v := range variations {
		b.Run(v.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				td := newBenchTraces(v)
				if err := exporter.ConsumeTraces(ctx, td); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkReceiverProcessorExporter adds the pipelines which processes the data.
func BenchmarkReceiverProcessorExporter(b *testing.B) {
	cfg := threeGroupConfig()
	require.NoError(b, cfg.Validate())

	exporter := consumertest.NewNop()
	factory := NewFactory()
	proc, err := factory.CreateTraces(
		context.Background(),
		processortest.NewNopSettings(component.MustNewType(typeStr)),
		cfg,
		exporter,
	)
	require.NoError(b, err)
	require.NoError(b, proc.Start(context.Background(), componenttest.NewNopHost()))
	b.Cleanup(func() { _ = proc.Shutdown(context.Background()) })

	var _ consumer.Traces = proc
	ctx := context.Background()

	for _, v := range variations {
		b.Run(v.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				td := newBenchTraces(v)
				if err := proc.ConsumeTraces(ctx, td); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

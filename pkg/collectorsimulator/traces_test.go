package collectorsimulator

import (
	"context"
	"testing"
	"time"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestTracesProcessingSimulation(t *testing.T) {
	require := require.New(t)

	inputTraces := []ptrace.Traces{
		makeTestPtrace("test span 1", map[string]string{
			"method": "GET",
		}),
		makeTestPtrace("test span 2", map[string]string{
			"method": "POST",
		}),
	}

	testAttributesConf1, err := yaml.Parser().Unmarshal([]byte(`
    include:
        match_type: strict
        attributes:
            - key: method
              value: GET
    actions:
        - key: test
          value: test-value-get
          action: insert
    `))
	require.Nil(err, "could not unmarshal test attributes processor config")
	testProcessor1 := ProcessorConfig{
		Name:   "attributes/test",
		Config: testAttributesConf1,
	}

	testAttributesConf2, err := yaml.Parser().Unmarshal([]byte(`
    include:
        match_type: strict
        attributes:
            - key: method
              value: POST
    actions:
        - key: test
          value: test-value-post
          action: insert
    `))
	require.Nil(err, "could not unmarshal test attributes processor config")
	testProcessor2 := ProcessorConfig{
		Name:   "attributes/test2",
		Config: testAttributesConf2,
	}

	processorFactories, err := otelcol.MakeFactoryMap(
		attributesprocessor.NewFactory(),
	)
	require.Nil(err, "could not create processors factory map")

	configGenerator := makeTestTracesConfigGenerator(
		[]ProcessorConfig{testProcessor1, testProcessor2},
	)
	outputTraces, collectorErrs, err := SimulateTracesProcessing(
		context.Background(),
		processorFactories,
		configGenerator,
		inputTraces,
		300*time.Millisecond,
	)
	require.Nil(err)
	require.Equal(0, len(collectorErrs))
	require.Equal(len(inputTraces), len(outputTraces))

	for _, td := range outputTraces {
		rs := td.ResourceSpans().At(0)
		ss := rs.ScopeSpans().At(0)
		span := ss.Spans().At(0)
		method, exists := span.Attributes().Get("method")
		require.True(exists)
		testVal, exists := span.Attributes().Get("test")
		require.True(exists)
		if method.Str() == "GET" {
			require.Equal("test-value-get", testVal.Str())
		} else {
			require.Equal("test-value-post", testVal.Str())
		}
	}
}

func makeTestPtrace(spanName string, attrsStr map[string]string) ptrace.Traces {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()

	scopeSpan := rs.ScopeSpans().AppendEmpty()
	span := scopeSpan.Spans().AppendEmpty()
	span.SetName(spanName)
	spanAttribs := span.Attributes()
	for k, v := range attrsStr {
		spanAttribs.PutStr(k, v)
	}

	return traces
}

func makeTestTracesConfigGenerator(
	processorConfigs []ProcessorConfig,
) ConfigGenerator {
	return func(baseConf []byte) ([]byte, error) {
		conf, err := yaml.Parser().Unmarshal([]byte(baseConf))
		if err != nil {
			return nil, err
		}

		processors := map[string]interface{}{}
		if conf["processors"] != nil {
			processors = conf["processors"].(map[string]interface{})
		}
		tracesProcessors := []string{}
		svc := conf["service"].(map[string]interface{})
		svcPipelines := svc["pipelines"].(map[string]interface{})
		svcTracesPipeline := svcPipelines["traces"].(map[string]interface{})
		if svcTracesPipeline["processors"] != nil {
			tracesProcessors = svcTracesPipeline["processors"].([]string)
		}

		for _, processorConf := range processorConfigs {
			processors[processorConf.Name] = processorConf.Config
			tracesProcessors = append(tracesProcessors, processorConf.Name)
		}

		conf["processors"] = processors
		svcTracesPipeline["processors"] = tracesProcessors

		confYaml, err := yaml.Parser().Marshal(conf)
		if err != nil {
			return nil, err
		}

		return confYaml, nil
	}
}

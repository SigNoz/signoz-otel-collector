package signozlogspipelineprocessor

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	"github.com/SigNoz/signoz-otel-collector/pkg/parser/grok"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor/processortest"
	"gopkg.in/yaml.v2"
)

func init() {

	grok.RegisterStanzaParser()
}

// Tests for processors (stanza operators) supported in log pipelines

func TestAddProcessor(t *testing.T) {
	confYaml := `
  operators:
    - type: add
      field: attributes.test
      value: testValue
  `

	input := []plog.Logs{makePlog(
		"test log", map[string]any{},
	)}
	expectedOutput := []plog.Logs{makePlog(
		"test log", map[string]any{"test": "testValue"},
	)}

	validateProcessorBehavior(t, confYaml, input, expectedOutput)
}

func TestRemoveProcessor(t *testing.T) {
	confYaml := `
  operators:
    - type: remove
      field: attributes.test
  `

	input := []plog.Logs{makePlog(
		"test log", map[string]any{"test": "testValue"},
	)}
	expectedOutput := []plog.Logs{makePlog(
		"test log", map[string]any{},
	)}

	validateProcessorBehavior(t, confYaml, input, expectedOutput)
}

func TestMoveProcessor(t *testing.T) {
	confYaml := `
  operators:
    - type: move
      from: attributes.test
      to: attributes.test1
  `

	input := []plog.Logs{makePlog(
		"test log", map[string]any{"test": "testValue"},
	)}
	expectedOutput := []plog.Logs{makePlog(
		"test log", map[string]any{"test1": "testValue"},
	)}

	validateProcessorBehavior(t, confYaml, input, expectedOutput)
}

func TestCopyProcessor(t *testing.T) {
	confYaml := `
  operators:
    - type: copy
      from: attributes.test
      to: attributes.test1
  `

	input := []plog.Logs{makePlog(
		"test log", map[string]any{"test": "testValue"},
	)}
	expectedOutput := []plog.Logs{makePlog(
		"test log", map[string]any{"test": "testValue", "test1": "testValue"},
	)}

	validateProcessorBehavior(t, confYaml, input, expectedOutput)
}

func TestRegexProcessor(t *testing.T) {
	confYaml := `
  operators:
    - type: regex_parser
      regex: ^a=(?P<a>.+);b=(?P<b>.+)$
      parse_from: body
      parse_to: attributes
  `

	input := []plog.Logs{makePlog(
		"a=aval;b=bval", map[string]any{},
	)}
	expectedOutput := []plog.Logs{makePlog(
		"a=aval;b=bval", map[string]any{
			"a": "aval",
			"b": "bval",
		},
	)}

	validateProcessorBehavior(t, confYaml, input, expectedOutput)
}

func TestGrokProcessor(t *testing.T) {
	confYaml := `
  operators:
    - type: grok_parser
      pattern: 'status: %{INT:status_code:int}'
      parse_from: body
      parse_to: attributes
  `

	input := []plog.Logs{makePlog(
		"status: 200", map[string]any{},
	)}
	expectedOutput := []plog.Logs{makePlog(
		"status: 200", map[string]any{"status_code": 200},
	)}

	validateProcessorBehavior(t, confYaml, input, expectedOutput)
}

func TestJSONProcessor(t *testing.T) {
	confYaml := `
  operators:
    - type: json_parser
      parse_from: body
      parse_to: attributes
  `

	input := []plog.Logs{makePlog(
		`{"status": "ok"}`, map[string]any{},
	)}
	expectedOutput := []plog.Logs{makePlog(
		`{"status": "ok"}`, map[string]any{"status": "ok"},
	)}

	validateProcessorBehavior(t, confYaml, input, expectedOutput)
}

func TestTraceProcessor(t *testing.T) {
	confYaml := `
  operators:
    - type: trace_parser
      trace_id:
        parse_from: attributes.traceId
  `

	input := []plog.Logs{makePlog(
		"test log", map[string]any{"traceId": "e37e734349000e2eda9c07cca0ceb692"},
	)}
	expectedOutput := []plog.Logs{makePlogWithTopLevelFields(
		t, "test log", map[string]any{"traceId": "e37e734349000e2eda9c07cca0ceb692"},
		map[string]any{
			"trace_id": "e37e734349000e2eda9c07cca0ceb692",
		},
	)}

	validateProcessorBehavior(t, confYaml, input, expectedOutput)
}

func TestSeverityProcessor(t *testing.T) {
	confYaml := `
  operators:
    - type: severity_parser
      parse_from: attributes.sev
      mapping:
        error: oops
      overwrite_text: true
  `

	input := []plog.Logs{makePlog(
		"test log", map[string]any{"sev": "oops"},
	)}
	expectedOutput := []plog.Logs{makePlogWithTopLevelFields(
		t, "test log", map[string]any{"sev": "oops"},
		map[string]any{
			"severity_text":   "ERROR",
			"severity_number": 17,
		},
	)}

	validateProcessorBehavior(t, confYaml, input, expectedOutput)
}

func TestTimeProcessor(t *testing.T) {
	confYaml := `
  operators:
    - type: time_parser
      parse_from: attributes.tsUnixEpoch
      layout_type: epoch
      layout: s
      overwrite_text: true
  `

	input := []plog.Logs{makePlog(
		"test log", map[string]any{"tsUnixEpoch": 9999},
	)}
	expectedOutput := []plog.Logs{makePlogWithTopLevelFields(
		t, "test log", map[string]any{"tsUnixEpoch": 9999},
		map[string]any{"timestamp": time.Unix(9999, 0)},
	)}

	validateProcessorBehavior(t, confYaml, input, expectedOutput)
}

func validateProcessorBehavior(
	t *testing.T,
	confYaml string,
	inputLogs []plog.Logs,
	expectedOutput []plog.Logs,
) {
	require := require.New(t)

	factory := NewFactory()

	config := parseProcessorConfig(t, confYaml)
	testSink := new(consumertest.LogsSink)
	ltp, err := factory.CreateLogsProcessor(
		context.Background(),
		processortest.NewNopCreateSettings(),
		config, testSink,
	)
	require.NoError(err)

	err = ltp.Start(context.Background(), nil)
	require.NoError(err)

	for _, inputPlog := range inputLogs {
		err = ltp.ConsumeLogs(context.Background(), inputPlog)
		require.NoError(err)
	}

	output := testSink.AllLogs()
	require.Len(output, len(expectedOutput))
	for i, expected := range expectedOutput {
		found := output[i]
		require.NoError(plogtest.CompareLogs(expected, found))
	}
}

func parseProcessorConfig(t *testing.T, confYaml string) component.Config {
	var rawConf map[string]any
	err := yaml.Unmarshal([]byte(confYaml), &rawConf)
	require.NoError(t, err, "couldn't parse config yaml")
	cm := confmap.NewFromStringMap(rawConf)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	require.NoError(
		t, component.UnmarshalConfig(cm, cfg),
		"couldn't unmarshal parsed yaml into processor config",
	)
	return cfg
}

func makePlog(body string, attributes map[string]any) plog.Logs {
	ld := plog.NewLogs()
	lr := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	lr.Body().SetStr(body)
	lr.Attributes().FromRaw(attributes)

	lr.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Unix(500, 0)))

	return ld
}

func makePlogWithTopLevelFields(t *testing.T, body string, attributes map[string]any, fields map[string]any) plog.Logs {
	ld := makePlog(body, attributes)
	lr := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)

	if traceId, exists := fields["trace_id"]; exists {
		traceIdBytes, err := hex.DecodeString(traceId.(string))
		require.NoError(t, err)

		lr.SetTraceID(pcommon.TraceID(traceIdBytes))
	}

	if sevText, exists := fields["severity_text"]; exists {
		lr.SetSeverityText(sevText.(string))
	}
	if sevNum, exists := fields["severity_number"]; exists {
		lr.SetSeverityNumber(plog.SeverityNumber(sevNum.(int)))
	}

	if timestamp, exists := fields["timestamp"]; exists {
		lr.SetTimestamp(pcommon.NewTimestampFromTime(timestamp.(time.Time)))
	}

	return ld
}

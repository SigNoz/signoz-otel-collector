package signozlogspipelineprocessor

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"
	"testing"
	"time"

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

// Test that translated Signoz logs pipelines work as expected
func TestSignozLogPipelineWithRouterOp(t *testing.T) {
	confYaml := `
  operators:
      - default: noop
        id: router_signoz
        routes:
          - expr: attributes["container_name"] == "hotrod"
            output: 61357da4-6d61-47f7-bcc5-1a2f00801b9a
        type: router
      - id: 61357da4-6d61-47f7-bcc5-1a2f00801b9a
        if: body != nil && body matches "^(?P<ts>.*)\\t(?P<log_level>.*)\\t(?P<location>.*)\\t(?P<msg>.*)\\t(?P<data_json>.*)$$"
        on_error: send
        output: f2ff87ad-f051-46d4-89c3-0cef056d8677
        parse_from: body
        parse_to: attributes
        regex: ^(?P<ts>.*)\t(?P<log_level>.*)\t(?P<location>.*)\t(?P<msg>.*)\t(?P<data_json>.*)$$
        type: regex_parser
      - id: f2ff87ad-f051-46d4-89c3-0cef056d8677
        if: attributes?.data_json != nil && attributes.data_json matches "^\\s*{.*}\\s*$$"
        output: dde26fcc-9140-4415-b1fb-d6daeab5e744
        parse_from: attributes.data_json
        parse_to: attributes
        type: json_parser
      - id: dde26fcc-9140-4415-b1fb-d6daeab5e744
        if: attributes?.log_level != nil && ( type(attributes.log_level) == "string" || ( type(attributes.log_level) in ["int", "float"] && attributes.log_level == float(int(attributes.log_level)) ) )
        mapping:
          debug:
              - DEBUG
          error:
              - ERROR
          fatal:
              - FATAL
          info:
              - INFO
          trace:
              - TRACE
          warn:
              - WARN
        overwrite_text: true
        parse_from: attributes.log_level
        type: severity_parser
      - id: 01ee2767-5d86-43c7-b0c4-a66651850b51
        type: remove
        if: attributes?.data_json != nil
        field: attributes.data_json
      - id: noop
        type: noop
  `

	// A matching log should get processed.
	input := []plog.Logs{makePlog(
		`2024-09-04T09:58:39.635Z	ERROR	driver/server.go:85	Retrying GetDriver after error	{"service": "driver", "trace_id": "738d1c34020ba19e", "span_id": "69e77f208cb24e9b", "retry_no": 1, "error": "redis timeout"}`,
		map[string]any{"container_name": "hotrod"},
	)}
	expectedOutput := []plog.Logs{makePlogWithTopLevelFields(
		t,
		`2024-09-04T09:58:39.635Z	ERROR	driver/server.go:85	Retrying GetDriver after error	{"service": "driver", "trace_id": "738d1c34020ba19e", "span_id": "69e77f208cb24e9b", "retry_no": 1, "error": "redis timeout"}`,
		map[string]any{
			"container_name": "hotrod",
			"error":          "redis timeout",
			"location":       "driver/server.go:85",
			"log_level":      "ERROR",
			"msg":            "Retrying GetDriver after error",
			"retry_no":       float64(1),
			"service":        "driver",
			"span_id":        "69e77f208cb24e9b",
			"trace_id":       "738d1c34020ba19e",
			"ts":             "2024-09-04T09:58:39.635Z",
		},
		map[string]any{
			"severity_text":   "ERROR",
			"severity_number": 17,
		},
	)}

	validateProcessorBehavior(t, confYaml, input, expectedOutput)

	// A non-matching log should get ignored.
	input = []plog.Logs{makePlog(
		`2024-09-04T09:58:39.635Z	ERROR	driver/server.go:85	Retrying GetDriver after error	{"service": "driver", "trace_id": "738d1c34020ba19e", "span_id": "69e77f208cb24e9b", "retry_no": 1, "error": "redis timeout"}`,
		map[string]any{"container_name": "not-hotrod"},
	)}
	expectedOutput = []plog.Logs{makePlog(
		`2024-09-04T09:58:39.635Z	ERROR	driver/server.go:85	Retrying GetDriver after error	{"service": "driver", "trace_id": "738d1c34020ba19e", "span_id": "69e77f208cb24e9b", "retry_no": 1, "error": "redis timeout"}`,
		map[string]any{"container_name": "not-hotrod"},
	)}
	validateProcessorBehavior(t, confYaml, input, expectedOutput)
}

// logstransform in otel-collector-contrib doesn't support using severity text and number in route expressions
func TestSeverityBasedRouteExpressions(t *testing.T) {
	for _, routeExpr := range []string{
		`severity_number == 9`,
		`severity_text == "INFO"`,
	} {
		confYaml := fmt.Sprintf(`
    operators:
        - default: noop
          id: router_signoz
          routes:
            - expr: %s
              output: add-test-value
          type: router
        - id: add-test-value
          on_error: send
          type: add
          field: attributes.test
          value: test-value
        - id: noop
          type: noop
    `, routeExpr)

		// should process matching log
		input := []plog.Logs{makePlogWithTopLevelFields(
			t, "test log", map[string]any{},
			map[string]any{"severity_text": "INFO", "severity_number": 9},
		)}
		expectedOutput := []plog.Logs{makePlogWithTopLevelFields(
			t, "test log", map[string]any{"test": "test-value"},
			map[string]any{"severity_text": "INFO", "severity_number": 9},
		)}
		validateProcessorBehavior(t, confYaml, input, expectedOutput)

		// should ignore non-matching log
		input = []plog.Logs{makePlogWithTopLevelFields(
			t, "test log", map[string]any{},
			map[string]any{"severity_text": "ERROR", "severity_number": 17},
		)}
		expectedOutput = []plog.Logs{makePlogWithTopLevelFields(
			t, "test log", map[string]any{},
			map[string]any{"severity_text": "ERROR", "severity_number": 17},
		)}
		validateProcessorBehavior(t, confYaml, input, expectedOutput)
	}
}

// When used with server based receivers like (http, otlp etc)
// ConsumeLogs will be called concurrently
func TestConcurrentConsumeLogs(t *testing.T) {
	require := require.New(t)

	factory := NewFactory()

	confYaml := `
  operators:
    - type: add
      field: attributes.test
      value: testValue
  `

	config := parseProcessorConfig(t, confYaml)
	testSink := new(consumertest.LogsSink)
	proc, err := factory.CreateLogsProcessor(
		context.Background(),
		processortest.NewNopCreateSettings(),
		config, testSink,
	)
	require.NoError(err)

	err = proc.Start(context.Background(), nil)
	require.NoError(err)

	var wg sync.WaitGroup

	processSomeLogs := func(count int) {
		defer wg.Done()

		for i := 0; i < count; i++ {
			testPlog := makePlog("test log", map[string]any{})
			consumeErr := proc.ConsumeLogs(context.Background(), testPlog)
			require.NoError(consumeErr)
		}
	}

	for j := 0; j < 10; j++ {
		wg.Add(1)
		go processSomeLogs(10)
	}

	wg.Wait()

	output := testSink.AllLogs()
	for _, l := range output {
		require.NoError(plogtest.CompareLogs(l, makePlog("test log", map[string]any{
			"test": "testValue",
		})))
	}
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
	proc, err := factory.CreateLogsProcessor(
		context.Background(),
		processortest.NewNopCreateSettings(),
		config, testSink,
	)
	require.NoError(err)

	err = proc.Start(context.Background(), nil)
	require.NoError(err)

	for _, inputPlog := range inputLogs {
		err = proc.ConsumeLogs(context.Background(), inputPlog)
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

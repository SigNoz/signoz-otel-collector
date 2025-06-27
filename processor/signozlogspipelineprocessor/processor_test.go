package signozlogspipelineprocessor

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/internal/metadata"
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
      span_id:
        parse_from: attributes.spanId
      trace_flags:
        parse_from: attributes.traceFlags
  `

	input := []plog.Logs{makePlog(
		"test log", map[string]any{
			"traceId":    "e37e734349000e2eda9c07cca0ceb692",
			"spanId":     "da9c07cca0ceb692",
			"traceFlags": "02",
		},
	)}
	expectedOutput := []plog.Logs{makePlogWithTopLevelFields(
		t, "test log", map[string]any{
			"traceId":    "e37e734349000e2eda9c07cca0ceb692",
			"spanId":     "da9c07cca0ceb692",
			"traceFlags": "02",
		},
		map[string]any{
			"trace_id":    "e37e734349000e2eda9c07cca0ceb692",
			"span_id":     "da9c07cca0ceb692",
			"trace_flags": "02",
		},
	)}

	validateProcessorBehavior(t, confYaml, input, expectedOutput)

	// trace id and span id should be padded with 0s to the left
	// if provided hex strings are not of the expected length
	// See https://github.com/SigNoz/signoz/issues/3859 for an example

	input = []plog.Logs{makePlog("test log", map[string]any{
		"traceId": "da9c07cca0ceb692",
		"spanId":  "ceb692",
	})}

	expectedOutput = []plog.Logs{makePlogWithTopLevelFields(
		t, "test log", map[string]any{
			"traceId": "da9c07cca0ceb692",
			"spanId":  "ceb692",
		},
		map[string]any{
			"trace_id": "0000000000000000da9c07cca0ceb692",
			"span_id":  "0000000000ceb692",
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
	proc, err := factory.CreateLogs(
		context.Background(),
		processortest.NewNopSettings(metadata.Type),
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

func TestBodyFieldReferencesWhenBodyIsJson(t *testing.T) {
	type testCase struct {
		name           string
		confYaml       string
		input          plog.Logs
		expectedOutput plog.Logs
	}
	testCases := []testCase{}

	// Collect test cases for each op that supports reading fields from JSON body

	// router op should be able to specify expressions referring to fields inside JSON body
	testConfWithRouter := `
  operators:
    - id: router_signoz
      type: router
      routes:
        - expr: body.request.id == "test"
          output: test-add
    - type: add
      id: test-add
      field: attributes.test
      value: test-value
  `
	testCases = append(testCases, testCase{
		"router/happy_case", testConfWithRouter,
		makePlog(`{"request": {"id": "test"}}`, map[string]any{}),
		makePlog(`{"request": {"id": "test"}}`, map[string]any{"test": "test-value"}),
	})
	testCases = append(testCases, testCase{
		"router/body_not_json", testConfWithRouter,
		makePlog(`test`, map[string]any{}),
		makePlog(`test`, map[string]any{}),
	})

	// Add op should be able to specify expressions referring to body JSON field
	testAddOpConf := `
  operators:
    - type: add
      if: body.request.id != nil
      field: attributes.request_id
      value: EXPR(body.request.id)
  `
	testCases = append(testCases, testCase{
		"add/happy_case", testAddOpConf,
		makePlog(`{"request": {"id": "test"}}`, map[string]any{}),
		makePlog(`{"request": {"id": "test"}}`, map[string]any{"request_id": "test"}),
	})
	testCases = append(testCases, testCase{
		"add/body_not_json", testAddOpConf,
		makePlog(`test`, map[string]any{}),
		makePlog(`test`, map[string]any{}),
	})

	// copy
	testCopyOpConf := `
  operators:
    - type: copy
      if: body.request.id != nil
      from: body.request.id
      to: attributes.request_id
  `
	testCases = append(testCases, testCase{
		"copy/happy_case", testCopyOpConf,
		makePlog(`{"request": {"id": "test"}}`, map[string]any{}),
		makePlog(`{"request": {"id": "test"}}`, map[string]any{"request_id": "test"}),
	})
	testCases = append(testCases, testCase{
		"copy/missing_field", testCopyOpConf,
		makePlog(`{"request": {"status": "test"}}`, map[string]any{}),
		makePlog(`{"request": {"status": "test"}}`, map[string]any{}),
	})
	testCases = append(testCases, testCase{
		"copy/body_not_json", testCopyOpConf,
		makePlog(`test`, map[string]any{}),
		makePlog(`test`, map[string]any{}),
	})

	// grok
	testGrokConf := `
  operators:
    - type: grok_parser
      if: body.request.status != nil
      pattern: 'status: %{INT:status_code:int}'
      parse_from: body.request.status
      parse_to: attributes
  `
	testCases = append(testCases, testCase{
		"copy/happy_case", testGrokConf,
		makePlog(`{"request": {"status": "status: 404"}}`, map[string]any{}),
		makePlog(`{"request": {"status": "status: 404"}}`, map[string]any{"status_code": 404}),
	})
	testCases = append(testCases, testCase{
		"copy/missing_field", testGrokConf,
		makePlog(`{"request": {"id": "test"}}`, map[string]any{}),
		makePlog(`{"request": {"id": "test"}}`, map[string]any{}),
	})
	testCases = append(testCases, testCase{
		"copy/body_not_json", testGrokConf,
		makePlog(`test`, map[string]any{}),
		makePlog(`test`, map[string]any{}),
	})

	// Regex
	testRegexConf := `
  operators:
    - type: regex_parser
      if: body.request.status != nil
      regex: "^status: (?P<status_code>[0-9]+)$"
      parse_from: body.request.status
      parse_to: attributes
  `
	testCases = append(testCases, testCase{
		"regex/happy_case", testRegexConf,
		makePlog(`{"request": {"status": "status: 404"}}`, map[string]any{}),
		makePlog(`{"request": {"status": "status: 404"}}`, map[string]any{"status_code": "404"}),
	})
	testCases = append(testCases, testCase{
		"regex/missing_field", testRegexConf,
		makePlog(`{"request": {"id": "test"}}`, map[string]any{}),
		makePlog(`{"request": {"id": "test"}}`, map[string]any{}),
	})
	testCases = append(testCases, testCase{
		"regex/body_not_json", testRegexConf,
		makePlog(`test`, map[string]any{}),
		makePlog(`test`, map[string]any{}),
	})

	// JSON
	testJSONConf := `
  operators:
    - type: json_parser
      if: body.request != nil
      parse_from: body.request
      parse_to: attributes
  `
	testCases = append(testCases, testCase{
		"json/happy_case", testJSONConf,
		makePlog(`{"request": "{\"status\": \"ok\"}"}`, map[string]any{}),
		makePlog(`{"request": "{\"status\": \"ok\"}"}`, map[string]any{"status": "ok"}),
	})
	testCases = append(testCases, testCase{
		"json/missing_field", testJSONConf,
		makePlog(`{"user": {"id": "test"}}`, map[string]any{}),
		makePlog(`{"user": {"id": "test"}}`, map[string]any{}),
	})
	testCases = append(testCases, testCase{
		"json/body_not_json", testJSONConf,
		makePlog(`test`, map[string]any{}),
		makePlog(`test`, map[string]any{}),
	})

	// time
	testTimeParserConf := `
  operators:
    - type: time_parser
      if: body.request.unix_ts != nil
      parse_from: body.request.unix_ts
      layout_type: epoch
      layout: s
  `
	testCases = append(testCases, testCase{
		"ts/happy_case", testTimeParserConf,
		makePlog(`{"request": {"unix_ts": 1000}}`, map[string]any{}),
		makePlogWithTopLevelFields(t, `{"request": {"unix_ts": 1000}}`, map[string]any{}, map[string]any{
			"timestamp": time.Unix(1000, 0),
		}),
	})
	testCases = append(testCases, testCase{
		"ts/missing_field", testTimeParserConf,
		makePlog(`{"user": {"id": "test"}}`, map[string]any{}),
		makePlog(`{"user": {"id": "test"}}`, map[string]any{}),
	})
	testCases = append(testCases, testCase{
		"ts/body_not_json", testTimeParserConf,
		makePlog(`test`, map[string]any{}),
		makePlog(`test`, map[string]any{}),
	})

	// Sev parser
	testSevParserConf := `
  operators:
    - type: severity_parser
      if: body.request.status != nil
      parse_from: body.request.status
      mapping:
        error: 404
      overwrite_text: true
  `
	testCases = append(testCases, testCase{
		"sev/happy_case", testSevParserConf,
		makePlog(`{"request": {"status": 404}}`, map[string]any{}),
		makePlogWithTopLevelFields(t, `{"request": {"status": 404}}`, map[string]any{}, map[string]any{
			"severity_text":   "ERROR",
			"severity_number": 17,
		}),
	})
	testCases = append(testCases, testCase{
		"sev/missing_field", testSevParserConf,
		makePlog(`{"user": {"id": "test"}}`, map[string]any{}),
		makePlog(`{"user": {"id": "test"}}`, map[string]any{}),
	})
	testCases = append(testCases, testCase{
		"sev/body_not_json", testSevParserConf,
		makePlog(`test`, map[string]any{}),
		makePlog(`test`, map[string]any{}),
	})

	// Trace parser
	testTraceParserConf := `
  operators:
    - type: trace_parser
      if: body.request.traceId != nil
      trace_id:
        parse_from: body.request.traceId
      span_id:
        parse_from: body.request.spanId
  `
	testCases = append(testCases, testCase{
		"trace_parser/happy_case", testTraceParserConf,
		makePlog(`{"request": {"traceId": "e37e734349000e2eda9c07cca0ceb692", "spanId": "da9c07cca0ceb692"}}`, map[string]any{}),
		makePlogWithTopLevelFields(t, `{"request": {"traceId": "e37e734349000e2eda9c07cca0ceb692", "spanId": "da9c07cca0ceb692"}}`, map[string]any{}, map[string]any{
			"trace_id": "e37e734349000e2eda9c07cca0ceb692",
			"span_id":  "da9c07cca0ceb692",
		}),
	})
	testCases = append(testCases, testCase{
		"trace_parser/missing_field", testTraceParserConf,
		makePlog(`{"user": {"id": "test"}}`, map[string]any{}),
		makePlog(`{"user": {"id": "test"}}`, map[string]any{}),
	})
	testCases = append(testCases, testCase{
		"trace_parser/body_not_json", testTraceParserConf,
		makePlog(`test`, map[string]any{}),
		makePlog(`test`, map[string]any{}),
	})

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			validateProcessorBehavior(
				t, tc.confYaml, []plog.Logs{tc.input}, []plog.Logs{tc.expectedOutput},
			)
		})
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
	proc, err := factory.CreateLogs(
		context.Background(),
		processortest.NewNopSettings(metadata.Type),
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
		t, cm.Unmarshal(cfg),
		"couldn't unmarshal parsed yaml into processor config",
	)
	return cfg
}

func makePlog(body string, attributes map[string]any) plog.Logs {
	ld := plog.NewLogs()
	lr := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	lr.Body().SetStr(body)
	_ = lr.Attributes().FromRaw(attributes)

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
	if spanId, exists := fields["span_id"]; exists {
		spanIdBytes, err := hex.DecodeString(spanId.(string))
		require.NoError(t, err)
		lr.SetSpanID(pcommon.SpanID(spanIdBytes))
	}
	if traceFlags, exists := fields["trace_flags"]; exists {
		flags, err := strconv.ParseUint(traceFlags.(string), 16, 64)
		require.NoError(t, err)
		lr.SetFlags(plog.LogRecordFlags(flags))
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

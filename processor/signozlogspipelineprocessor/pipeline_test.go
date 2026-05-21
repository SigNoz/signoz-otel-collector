package signozlogspipelineprocessor

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/internal/metadata"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor/processortest"
)

var pipelineTestConf string

func init() {
	b, err := os.ReadFile("testdata/pipeline_test_config.yaml")
	if err != nil {
		panic("failed to read pipeline_test_config.yaml: " + err.Error())
	}
	pipelineTestConf = string(b)
}

// runPipeline starts a processor with confYaml, sends input, waits until
// wantRecords log records arrive, then returns all collected log records.
func runPipeline(t *testing.T, confYaml string, input plog.Logs, wantRecords int) []plog.LogRecord {
	t.Helper()

	factory := NewFactory()
	config := parseProcessorConfig(t, confYaml)
	sink := new(consumertest.LogsSink)
	proc, err := factory.CreateLogs(
		context.Background(),
		processortest.NewNopSettings(metadata.Type),
		config, sink,
	)
	require.NoError(t, err)
	require.NoError(t, proc.Start(context.Background(), nil))
	t.Cleanup(func() { _ = proc.Shutdown(context.Background()) })

	require.NoError(t, proc.ConsumeLogs(context.Background(), input))

	require.Eventually(t, func() bool {
		return sink.LogRecordCount() >= wantRecords
	}, 5*time.Second, 10*time.Millisecond, "timed out waiting for %d log records", wantRecords)

	var records []plog.LogRecord
	for _, ld := range sink.AllLogs() {
		for i := 0; i < ld.ResourceLogs().Len(); i++ {
			rl := ld.ResourceLogs().At(i)
			for j := 0; j < rl.ScopeLogs().Len(); j++ {
				sl := rl.ScopeLogs().At(j)
				for k := 0; k < sl.LogRecords().Len(); k++ {
					records = append(records, sl.LogRecords().At(k))
				}
			}
		}
	}
	require.Len(t, records, wantRecords)
	return records
}

func TestPipelineWithRouterAndRegex(t *testing.T) {
	testCases := []struct {
		name       string
		inputAttrs map[string]any
		wantAttrs  map[string]string
		wantAbsent []string
	}{
		{
			name:       "entry_not_matching_pipeline_filter",
			inputAttrs: map[string]any{"service": "my-service"},
			wantAttrs:  map[string]string{"service": "my-service"},
			wantAbsent: []string{"k8s_namespace_name"},
		},
		{
			name:       "entry_matching_filter_but_not_regex_step",
			inputAttrs: map[string]any{"log_tags": "env:prod,service:web"},
			wantAttrs:  map[string]string{"log_tags": "env:prod,service:web"},
			wantAbsent: []string{"k8s_namespace_name"},
		},
		{
			name:       "entry_matching_filter_and_all_steps",
			inputAttrs: map[string]any{"log_tags": "env:prod,kube_namespace:my-namespace,service:web"},
			wantAttrs:  map[string]string{"k8s_namespace_name": "my-namespace"},
			wantAbsent: []string{"log_tags"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			records := runPipeline(t, pipelineTestConf, makePlog("test log", tc.inputAttrs), 1)
			lr := records[0]

			require.Equal(t, "test log", lr.Body().Str())
			for k, want := range tc.wantAttrs {
				v, ok := lr.Attributes().Get(k)
				require.True(t, ok, "expected attribute %q to be present", k)
				require.Equal(t, want, v.Str())
			}
			for _, k := range tc.wantAbsent {
				_, ok := lr.Attributes().Get(k)
				require.False(t, ok, "attribute %q must not be present", k)
			}
		})
	}
}

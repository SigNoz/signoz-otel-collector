package signozmetricreductionrulesextension

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	cmock "github.com/srikanthccv/ClickHouse-go-mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/SigNoz/signoz-otel-collector/extension/signozmetricreductionrulesextension/internal/metadatatest"
)

func newTestExtension(t *testing.T, tt *componenttest.Telemetry, logger *zap.Logger) (*rulesExtension, cmock.ClickConnMockCommon) {
	conn, err := cmock.NewClickHouseWithQueryMatcher(nil, sqlmock.QueryMatcherRegexp)
	require.NoError(t, err)
	settings := tt.NewTelemetrySettings()
	settings.Logger = logger
	e, err := newExtension(createDefaultConfig().(*Config), settings, conn)
	require.NoError(t, err)
	return e, conn
}

func TestRulesExtension_Poll(t *testing.T) {
	testCases := []struct {
		name          string
		rows          [][]any
		metricName    string
		effectiveFrom int64
		droppedLabel  string
		keptLabel     string
		expectedLen   int
		expectedLogs  []map[string]any
	}{
		{
			name:          "MatchTypeDrop_Loaded",
			rows:          [][]any{{"http.server.duration0.bucket", []string{"pod"}, "drop", int64(1700000000000), false}},
			metricName:    "http.server.duration0.bucket",
			effectiveFrom: 1700000000000, droppedLabel: "pod", keptLabel: "service.name", expectedLen: 1,
		},
		{
			name:          "MatchTypeEmpty_LoadedAsDrop",
			rows:          [][]any{{"http.server.duration0.count", []string{"pod"}, "", int64(1700000060000), false}},
			metricName:    "http.server.duration0.count",
			effectiveFrom: 1700000060000, droppedLabel: "pod", keptLabel: "service.name", expectedLen: 1,
		},
		{
			name:          "MatchTypeKeep_NoLabels_KeepsOnlyProtected",
			rows:          [][]any{{"system.memory.usage0", []string{}, "keep", int64(1700000120000), false}},
			metricName:    "system.memory.usage0",
			effectiveFrom: 1700000120000, droppedLabel: "pod", keptLabel: "deployment.environment", expectedLen: 1,
		},
		{
			name:          "MatchTypeKeep_ProtectedLabel_Loaded",
			rows:          [][]any{{"zk.duration0.quantile", []string{"quantile"}, "keep", int64(1700000180000), false}},
			metricName:    "zk.duration0.quantile",
			effectiveFrom: 1700000180000, droppedLabel: "pod", keptLabel: "quantile", expectedLen: 1,
		},
		{
			name: "InvalidRowsAmongValid_OnlyValidLoaded",
			rows: [][]any{
				{"zk.duration0.sum", []string{"pod"}, "drop", int64(1700000540000), true},
				{"zk.duration0.count", []string{"pod"}, "KEEP", int64(1700000540000), false},
				{"system.cpu.time0", []string{"pod"}, "drop", int64(1700000540000), false},
				{"http.server.duration0.min", []string{}, "drop", int64(1700000540000), false},
				{"http.server.duration0.max", []string{"quantile"}, "drop", int64(1700000540000), false},
			},
			metricName:    "system.cpu.time0",
			effectiveFrom: 1700000540000, droppedLabel: "pod", keptLabel: "service.name",
			// 5 rows - deleted (1) - unknown match_type, case-sensitive (1) - drop with no labels (1) - drops protected label (1) = 1
			expectedLen: 1,
			expectedLogs: []map[string]any{
				{"msg": "ignoring reduction rule with unknown match_type", "metric_name": "zk.duration0.count", "match_type": "KEEP"},
				{"msg": "ignoring reduction rule that drops a protected label", "metric_name": "http.server.duration0.max", "label": "quantile"},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			tt := componenttest.NewTelemetry()
			t.Cleanup(func() { require.NoError(t, tt.Shutdown(context.Background())) })
			core, logs := observer.New(zapcore.WarnLevel)
			e, conn := newTestExtension(t, tt, zap.New(core))
			conn.ExpectQuery(`FROM signoz_metrics\.distributed_metric_reduction_rules\s+GROUP BY metric_name`).WillReturnRows(cmock.NewRows([]cmock.ColumnType{
				{Name: "metric_name", Type: "String"},
				{Name: "labels", Type: "Array(String)"},
				{Name: "match_type", Type: "String"},
				{Name: "effective_from_unix_milli", Type: "Int64"},
				{Name: "deleted", Type: "Bool"},
			}, testCase.rows))

			e.pollReductionRules(context.Background())

			rule := e.Current()[testCase.metricName]
			require.NotNil(t, rule)
			assert.False(t, rule.AppliesAt(testCase.effectiveFrom-1))
			assert.True(t, rule.AppliesAt(testCase.effectiveFrom))
			assert.True(t, rule.Drop(testCase.droppedLabel))
			assert.False(t, rule.Drop(testCase.keptLabel))
			assert.Len(t, e.Current(), testCase.expectedLen)
			var entries []map[string]any
			for _, entry := range logs.All() {
				fields := entry.ContextMap()
				fields["msg"] = entry.Message
				entries = append(entries, fields)
			}
			assert.Equal(t, testCase.expectedLogs, entries)
			metadatatest.AssertEqualSignozmetricreductionrulesRuleCount(t, tt, []metricdata.DataPoint[int64]{{Value: int64(testCase.expectedLen)}}, metricdatatest.IgnoreTimestamp())
			metadatatest.AssertEqualSignozmetricreductionrulesPolls(t, tt, []metricdata.DataPoint[int64]{{Value: 1}}, metricdatatest.IgnoreTimestamp())
			assert.NoError(t, conn.ExpectationsWereMet())
		})
	}
}

func TestRulesExtension_PollFails_LastRulesKept(t *testing.T) {
	testCases := []struct {
		name              string
		rows              *cmock.Rows
		err               error
		expectedErrorType string
	}{
		{name: "QueryError", err: errors.New("connection refused"), expectedErrorType: "other"},
		{name: "QueryTimeout", err: context.DeadlineExceeded, expectedErrorType: "timeout"},
		{
			name:              "ScanTypeMismatch",
			expectedErrorType: "other",
			rows: cmock.NewRows([]cmock.ColumnType{
				{Name: "metric_name", Type: "String"},
				{Name: "labels", Type: "Array(String)"},
				{Name: "match_type", Type: "String"},
				{Name: "effective_from_unix_milli", Type: "String"},
				{Name: "deleted", Type: "Bool"},
			}, [][]any{{"system.cpu.time0", []string{"pod"}, "drop", "1700000000000", false}}),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			tt := componenttest.NewTelemetry()
			t.Cleanup(func() { require.NoError(t, tt.Shutdown(context.Background())) })
			e, conn := newTestExtension(t, tt, zap.NewNop())
			conn.ExpectQuery(`FROM signoz_metrics\.distributed_metric_reduction_rules\s+GROUP BY metric_name`).WillReturnRows(cmock.NewRows([]cmock.ColumnType{
				{Name: "metric_name", Type: "String"},
				{Name: "labels", Type: "Array(String)"},
				{Name: "match_type", Type: "String"},
				{Name: "effective_from_unix_milli", Type: "Int64"},
				{Name: "deleted", Type: "Bool"},
			}, [][]any{
				{"http.server.duration0.bucket", []string{"pod"}, "drop", int64(0), false},
				{"zk.duration0.quantile", []string{"service.name"}, "keep", int64(0), false},
			}))
			conn.ExpectQuery(`FROM signoz_metrics\.distributed_metric_reduction_rules\s+GROUP BY metric_name`).WillReturnRows(testCase.rows).WillReturnError(testCase.err)

			assert.Nil(t, e.Current())
			e.pollReductionRules(context.Background())
			good := e.reductionRules.Load()
			e.pollReductionRules(context.Background())

			assert.Same(t, good, e.reductionRules.Load())
			// 2 rows, both valid
			assert.Len(t, e.Current(), 2)
			metadatatest.AssertEqualSignozmetricreductionrulesRuleCount(t, tt, []metricdata.DataPoint[int64]{{Value: 2}}, metricdatatest.IgnoreTimestamp())
			// 1 successful poll without error.type + 1 failed poll with it
			metadatatest.AssertEqualSignozmetricreductionrulesPolls(t, tt, []metricdata.DataPoint[int64]{
				{Value: 1},
				{Value: 1, Attributes: attribute.NewSet(attribute.String("error.type", testCase.expectedErrorType))},
			}, metricdatatest.IgnoreTimestamp())
			assert.NoError(t, conn.ExpectationsWereMet())
		})
	}
}

func TestRulesExtension_Shutdown_ClosesConnection(t *testing.T) {
	testCases := []struct {
		name    string
		enabled bool
		start   bool
	}{
		{name: "Enabled_WithoutStart", enabled: true, start: false},
		{name: "Disabled_StartRunsNoPoll", enabled: false, start: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			tt := componenttest.NewTelemetry()
			t.Cleanup(func() { require.NoError(t, tt.Shutdown(context.Background())) })
			e, conn := newTestExtension(t, tt, zap.NewNop())
			e.cfg.Enabled = testCase.enabled
			if testCase.start {
				require.NoError(t, e.Start(context.Background(), componenttest.NewNopHost()))
			}
			conn.ExpectClose()

			require.NoError(t, e.Shutdown(context.Background()))

			assert.Equal(t, testCase.enabled, e.Enabled())
			assert.Nil(t, e.Current())
			_, err := tt.GetMetric("otelcol.signozmetricreductionrules.polls")
			assert.Error(t, err, "no poll ran")
			assert.NoError(t, conn.ExpectationsWereMet())
		})
	}
}

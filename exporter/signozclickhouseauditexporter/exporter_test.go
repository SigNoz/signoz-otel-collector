package signozclickhouseauditexporter

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jellydator/ttlcache/v3"
	cmock "github.com/srikanthccv/ClickHouse-go-mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/SigNoz/signoz-otel-collector/exporter/signozclickhouseauditexporter/internal/metadata"
	"github.com/SigNoz/signoz-otel-collector/exporter/signozclickhouseauditexporter/internal/metadatatest"
	"github.com/SigNoz/signoz-otel-collector/utils"
)

func newTestExporter(t *testing.T) *auditExporter {
	t.Helper()

	exp, err := newExporter(exportertest.NewNopSettings(metadata.Type), createDefaultConfig().(*Config))
	require.NoError(t, err)

	return exp
}

func auditLogs() plog.Logs {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "signoz")
	record := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	record.SetTimestamp(1790730000000000000)
	record.Body().SetStr("user23 deleted alert-rule")
	record.Attributes().PutStr("signoz.audit.action", "delete")
	return ld
}

func TestPushLogsData(t *testing.T) {
	testCases := []struct {
		name            string
		cached          bool
		sendErr         error
		wantTables      []string
		wantErr         string
		wantKeysCached  bool
		wantDurationFor []string
	}{
		{
			name:            "EmptyCaches_WritesAllTables",
			wantTables:      []string{distributedLogsTable, distributedLogsResourceTable, distributedFieldKeysTable, distributedFieldsTable},
			wantKeysCached:  true,
			wantDurationFor: []string{distributedFieldKeysTable, distributedFieldsTable, distributedLogsTable, distributedLogsResourceTable},
		},
		{
			name:            "CachedMetadata_WritesLogsAndFields",
			cached:          true,
			wantTables:      []string{distributedLogsTable, distributedFieldsTable},
			wantKeysCached:  true,
			wantDurationFor: []string{distributedFieldsTable, distributedLogsTable},
		},
		{
			name:            "SendFails_ReturnsErrorAndSkipsCaches",
			sendErr:         errors.New("connection reset"),
			wantTables:      []string{distributedLogsTable, distributedLogsResourceTable, distributedFieldKeysTable, distributedFieldsTable},
			wantErr:         "connection reset",
			wantDurationFor: []string{distributedFieldKeysTable, distributedFieldsTable, distributedLogsResourceTable},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			tel := componenttest.NewTelemetry()
			t.Cleanup(func() { require.NoError(t, tel.Shutdown(context.Background())) })

			exp, err := newExporter(metadatatest.NewSettings(tel), createDefaultConfig().(*Config))
			require.NoError(t, err)

			mock, err := cmock.NewClickHouseWithQueryMatcher(nil, sqlmock.QueryMatcherRegexp)
			require.NoError(t, err)
			mock.MatchExpectationsInOrder(false)
			exp.conn = mock

			if testCase.cached {
				exp.keysCache.Set("service.name:resource:string", struct{}{}, ttlcache.DefaultTTL)
				exp.keysCache.Set("signoz.audit.action:attribute:string", struct{}{}, ttlcache.DefaultTTL)
				exp.rfCache.Set(utils.MakeKeyForRFCache(1790730000, resourceFingerprint(map[string]any{"service.name": "signoz"})), struct{}{}, ttlcache.DefaultTTL)
			}

			for _, table := range testCase.wantTables {
				batch := mock.ExpectPrepareBatch(`INSERT INTO signoz_audit\.` + table + ` \(`)
				batch.ExpectAppend()
				send := batch.ExpectSend()
				if table == distributedLogsTable && testCase.sendErr != nil {
					send.WillReturnError(testCase.sendErr)
				}
			}

			err = exp.pushLogsData(context.Background(), auditLogs())
			if testCase.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), testCase.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
			assert.Equal(t, testCase.wantKeysCached, exp.keysCache.Has("signoz.audit.action:attribute:string"))

			got, err := tel.GetMetric("otelcol.signozclickhouseaudit.write.duration")
			require.NoError(t, err)
			histogram, ok := got.Data.(metricdata.Histogram[float64])
			require.True(t, ok)
			var tables []string
			for _, dp := range histogram.DataPoints {
				table, _ := dp.Attributes.Value(attribute.Key("table"))
				tables = append(tables, table.AsString())
				assert.Equal(t, uint64(1), dp.Count)
			}
			assert.ElementsMatch(t, testCase.wantDurationFor, tables)
		})
	}
}

package signozclickhousemetrics

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	cmock "github.com/srikanthccv/ClickHouse-go-mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/otel/metric/noop"
	"go.uber.org/zap"

	internalmetadata "github.com/SigNoz/signoz-otel-collector/exporter/signozclickhousemetrics/internal/metadata"
	pkgfingerprint "github.com/SigNoz/signoz-otel-collector/internal/common/fingerprint"
	"github.com/SigNoz/signoz-otel-collector/pkg/pdatagen/pmetricsgen"
)

func newRegistrationExporter(t *testing.T, enabled bool, opts ...ExporterOption) *clickhouseMetricsExporter {
	t.Helper()
	cfg := &Config{MetadataWriteSampleRatio: 1}
	cfg.TimeBucketedSet.Enabled = enabled
	cfg.TimeBucketedSet.MaxBucketSize = 32 << 20
	cfg.TimeBucketedSet.PreWriteWindow = 15 * time.Minute
	exp, err := NewClickHouseExporter(append([]ExporterOption{
		WithLogger(zap.NewNop()),
		WithConfig(cfg),
		WithMeter(noop.NewMeterProvider().Meter(internalmetadata.ScopeName)),
		WithSettings(exportertest.NewNopSettings(internalmetadata.Type)),
	}, opts...)...)
	require.NoError(t, err)
	return exp
}

func Test_planTimeSeries(t *testing.T) {
	bucketStart := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC).UnixMilli()
	midBucket := bucketStart + 20*time.Minute.Milliseconds()
	lastMilliOfBucket := bucketStart + time.Hour.Milliseconds() - 1

	pointAttrs := pcommon.NewMap()
	pointAttrs.PutStr("host", "a")
	point := pkgfingerprint.NewFingerprint(pkgfingerprint.PointFingerprintType, 0, pointAttrs, map[string]string{"__temporality__": "Unspecified"})
	scopeAttrs := map[string]string{"__scope.name__": "test"}
	resourceAttrs := map[string]string{"service.name": "svc"}
	reduced := &reducedSeries{fingerprint: 77, point: point, scope: point, resource: point}

	row := ts{metricName: "http.requests", fingerprint: 42, unixMilli: bucketStart + 5*time.Minute.Milliseconds()}

	plan := func(b *batch, nowMilli int64, reducer *reducer, reduced *reducedSeries) {
		b.nowMilli = nowMilli
		b.planTimeSeries(row, point, scopeAttrs, resourceAttrs, reducer, reduced)
	}
	newTestBatch := func(exp *clickhouseMetricsExporter) *batch {
		return newBatch(zap.NewNop(), exp.timeSeriesTimeBucketedSet, 0, 0, 0)
	}

	type wantRow struct {
		isReduced    bool
		writeCurrent bool
		writeNext    bool
	}

	testCases := []struct {
		name    string
		enabled bool
		run     func(exp *clickhouseMetricsExporter) *batch
		want    []wantRow
	}{
		{
			name: "Disabled_RepeatInBatch_EveryRowBuilt",
			run: func(exp *clickhouseMetricsExporter) *batch {
				b := newTestBatch(exp)
				plan(b, midBucket, nil, nil)
				plan(b, midBucket, nil, nil)
				return b
			},
			want: []wantRow{{}, {}},
		},
		{
			name: "Disabled_Reduced_OneReducedRowPerReducer",
			run: func(exp *clickhouseMetricsExporter) *batch {
				b := newTestBatch(exp)
				r := &reducer{}
				plan(b, midBucket, r, reduced)
				plan(b, midBucket, r, reduced)
				return b
			},
			want: []wantRow{{}, {isReduced: true}, {}},
		},
		{
			name:    "Enabled_FirstSeen_WritesCurrent",
			enabled: true,
			run: func(exp *clickhouseMetricsExporter) *batch {
				b := newTestBatch(exp)
				plan(b, midBucket, nil, nil)
				return b
			},
			want: []wantRow{{writeCurrent: true}},
		},
		{
			name:    "Enabled_RepeatInBatch_PlannedOnce",
			enabled: true,
			run: func(exp *clickhouseMetricsExporter) *batch {
				b := newTestBatch(exp)
				plan(b, midBucket, nil, nil)
				plan(b, midBucket, nil, nil)
				return b
			},
			want: []wantRow{{writeCurrent: true}},
		},
		{
			name:    "Enabled_Registered_NoRow",
			enabled: true,
			run: func(exp *clickhouseMetricsExporter) *batch {
				first := newTestBatch(exp)
				plan(first, midBucket, nil, nil)
				exp.timeSeriesTimeBucketedSet.Apply(registeredRows(first.ts))
				second := newTestBatch(exp)
				plan(second, midBucket, nil, nil)
				return second
			},
			want: nil,
		},
		{
			name:    "Enabled_Registered_InPreWriteWindow_WritesNext",
			enabled: true,
			run: func(exp *clickhouseMetricsExporter) *batch {
				first := newTestBatch(exp)
				plan(first, midBucket, nil, nil)
				exp.timeSeriesTimeBucketedSet.Apply(registeredRows(first.ts))
				second := newTestBatch(exp)
				plan(second, lastMilliOfBucket, nil, nil)
				return second
			},
			want: []wantRow{{writeNext: true}},
		},
		{
			name:    "Enabled_Reduced_RawAndReducedRowsDistinct",
			enabled: true,
			run: func(exp *clickhouseMetricsExporter) *batch {
				b := newTestBatch(exp)
				plan(b, midBucket, &reducer{}, reduced)
				plan(b, midBucket, &reducer{}, reduced)
				return b
			},
			want: []wantRow{{writeCurrent: true}, {isReduced: true, writeCurrent: true}},
		},
		{
			name:    "Enabled_Reduced_Registered_NoRow",
			enabled: true,
			run: func(exp *clickhouseMetricsExporter) *batch {
				first := newTestBatch(exp)
				plan(first, midBucket, &reducer{}, reduced)
				exp.timeSeriesTimeBucketedSet.Apply(registeredRows(first.ts))
				second := newTestBatch(exp)
				plan(second, midBucket, &reducer{}, reduced)
				return second
			},
			want: nil,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			exp := newRegistrationExporter(t, testCase.enabled)
			b := testCase.run(exp)
			require.Len(t, b.ts, len(testCase.want))
			for i, want := range testCase.want {
				got := b.ts[i]
				assert.Equal(t, want.isReduced, got.isReduced, "row %d isReduced", i)
				assert.Equal(t, want.writeCurrent, got.writeCurrent, "row %d writeCurrent", i)
				assert.Equal(t, want.writeNext, got.writeNext, "row %d writeNext", i)
				assert.Equal(t, bucketStart, got.bucketStart, "row %d bucketStart", i)
				assert.NotEmpty(t, got.labels, "row %d labels", i)
				assert.NotEmpty(t, got.attrs, "row %d attrs", i)
			}
		})
	}
}

func Test_writeBatchMarksSeriesOnlyAfterSend(t *testing.T) {
	testCases := []struct {
		name                string
		enabled             bool
		sendErr             error
		wantTsOnSecondBatch int
	}{
		{name: "Disabled_SecondBatchPlansEveryRow", wantTsOnSecondBatch: 1},
		{name: "Enabled_SendSucceeds_SecondBatchSkipsRegisteredSeries", enabled: true, wantTsOnSecondBatch: 0},
		{name: "Enabled_SendFails_SecondBatchReplansSeries", enabled: true, sendErr: errors.New("send failed"), wantTsOnSecondBatch: 1},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			conn, err := cmock.NewClickHouseNative(nil)
			require.NoError(t, err)
			conn.MatchExpectationsInOrder(false)
			conn.ExpectPrepareBatch(fmt.Sprintf(samplesSQLTmpl, "", ""))
			timeSeriesBatch := conn.ExpectPrepareBatch(fmt.Sprintf(timeSeriesSQLTmpl, "", ""))
			if testCase.sendErr != nil {
				timeSeriesBatch.ExpectSend().WillReturnError(testCase.sendErr)
			}
			conn.ExpectPrepareBatch(fmt.Sprintf(expHistSQLTmpl, "", ""))
			conn.ExpectPrepareBatch(fmt.Sprintf(metadataSQLTmpl, "", ""))

			exp := newRegistrationExporter(t, testCase.enabled, WithConn(conn))

			metrics := pmetricsgen.GenerateGaugeMetrics(1, 1, 1, 1, 1, 0, 0)
			first := exp.prepareBatch(context.Background(), metrics)
			require.Len(t, first.ts, 1)
			assert.Equal(t, testCase.enabled, first.ts[0].writeCurrent)

			err = exp.writeBatch(context.Background(), first)
			if testCase.sendErr != nil {
				require.ErrorIs(t, err, testCase.sendErr)
			} else {
				require.NoError(t, err)
			}

			second := exp.prepareBatch(context.Background(), metrics)
			assert.Len(t, second.samples, 1)
			assert.Len(t, second.ts, testCase.wantTsOnSecondBatch)
		})
	}
}

package signozclickhousemeter

import (
	"context"
	"log"
	"testing"

	cmock "github.com/srikanthccv/ClickHouse-go-mock"
	"go.uber.org/zap/zaptest"

	"github.com/SigNoz/signoz-otel-collector/pkg/pdatagen/pmetricsgen"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func Benchmark_prepareBatchSum(b *testing.B) {
	// 10k sum * 10 data points = 100k data point in total
	// each with 30 total attributes
	metrics := pmetricsgen.GenerateSumMetrics(10000, 10, 10, 10, 10, 0, 0)
	b.ResetTimer()
	b.ReportAllocs()
	exp, err := NewClickHouseExporter(zap.NewNop(), &Config{DSN: "tcp://localhost:9000?database=default"})
	require.NoError(b, err)
	for i := 0; i < b.N; i++ {
		exp.prepareBatch(context.Background(), metrics)
	}
	err = exp.Shutdown(context.Background())
	require.NoError(b, err)
}

func Test_prepareBatchSumWithNoRecordedValue(t *testing.T) {
	metrics := pmetricsgen.GenerateSumMetrics(1, 1, 1, 1, 1, 1, 0)
	exp, err := NewClickHouseExporter(zap.NewNop(), &Config{DSN: "tcp://localhost:9000?database=default"})
	require.NoError(t, err)
	batch := exp.prepareBatch(context.Background(), metrics)
	assert.NotNil(t, batch)
	expectedSamples := []sample{
		{
			temporality: pmetric.AggregationTemporalityCumulative,
			metricName:  "system.cpu.time0",
			unixMilli:   1727286182000,
			value:       0,
			description: "cpu time of the host",
			unit:        "s",
			typ:         pmetric.MetricTypeSum,
			isMonotonic: true,
			labels:      "{\"__name__\":\"system.cpu.time0\",\"__scope.name__\":\"go.signoz.io/app/reader\",\"__scope.schema_url__\":\"scope.schema_url\",\"__scope.version__\":\"1.0.0\",\"__temporality__\":\"Cumulative\",\"resource.attr_0\":\"value0\",\"scope.attr_0\":\"value0\",\"sum.attr_0\":\"1\"}",
		},
	}
	assert.Equal(t, len(expectedSamples), len(batch.samples))

	for idx, sample := range expectedSamples {
		curSample := batch.samples[idx]
		assert.Equal(t, sample.temporality, curSample.temporality)
		assert.Equal(t, sample.metricName, curSample.metricName)
		assert.Equal(t, sample.unixMilli, curSample.unixMilli)
		assert.Equal(t, sample.value, curSample.value)
	}
	err = exp.Shutdown(context.Background())
	assert.NoError(t, err)
}

func Test_prepareBatchSumWithNan(t *testing.T) {
	metrics := pmetricsgen.GenerateSumMetrics(1, 1, 1, 1, 1, 0, 1)
	exp, err := NewClickHouseExporter(zap.NewNop(), &Config{DSN: "tcp://localhost:9000?database=default"})
	require.NoError(t, err)
	batch := exp.prepareBatch(context.Background(), metrics)
	assert.Equal(t, 0, len(batch.samples))
	err = exp.Shutdown(context.Background())
	assert.NoError(t, err)
}

func Test_shutdown(t *testing.T) {
	conn, err := cmock.NewClickHouseNative(nil)
	if err != nil {
		log.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	conn.MatchExpectationsInOrder(false)
	conn.ExpectPrepareBatch("INSERT INTO . (temporality, metric_name, description, unit, type, is_monotonic, labels, fingerprint, unix_milli, value) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)") //samples query
	conn.ExpectClose()
	exporter, err := NewClickHouseExporter(zaptest.NewLogger(t), &Config{DSN: "tcp://localhost:9000?database=default"})
	defaultConn := exporter.conn
	exporter.conn = conn

	if err != nil {
		log.Fatalf("an error '%s' was not expected when creating new exporter", err)
	}

	// Send one metric before shutdown
	metrics := pmetricsgen.GenerateSumMetrics(1, 1, 1, 1, 1, 0, 0)
	err = exporter.ConsumeMetrics(context.Background(), metrics)
	if err != nil {
		t.Fatalf("unexpected error pushing metrics: %v", err)
	}

	err = exporter.Shutdown(context.Background())
	if err != nil {
		log.Fatalf("an error '%s' was not expected when shutting down exporter", err)
	}
	defaultConn.Close()
}

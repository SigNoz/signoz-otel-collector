package clickhouselogsexporter

import (
	"context"
	"log"
	"testing"
	"time"

	driver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/SigNoz/signoz-otel-collector/pkg/pdatagen/plogsgen"
	"github.com/google/uuid"
	"github.com/jellydator/ttlcache/v3"
	cmock "github.com/srikanthccv/ClickHouse-go-mock"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/otel/metric/noop"
	"go.uber.org/zap"

	"github.com/SigNoz/signoz-otel-collector/exporter/clickhouselogsexporter/internal/metadata"
)

func eventually(t *testing.T, f func() bool) {
	assert.Eventually(t, f, 10*time.Second, 100*time.Millisecond)
}

func testOptions() []LogExporterOption {
	// keys cache is used to avoid duplicate inserts for the same attribute key.
	keysCache := ttlcache.New(
		ttlcache.WithTTL[string, struct{}](240*time.Minute),
		ttlcache.WithCapacity[string, struct{}](50000),
	)
	go keysCache.Start()

	// resource fingerprint cache is used to avoid duplicate inserts for the same resource fingerprint.
	// the ttl is set to the same as the bucket rounded value i.e 1800 seconds.
	// if a resource fingerprint is seen in the bucket already, skip inserting it again.
	rfCache := ttlcache.New(
		ttlcache.WithTTL[string, struct{}](distributedLogsResourceV2Seconds*time.Second),
		ttlcache.WithDisableTouchOnHit[string, struct{}](),
		ttlcache.WithCapacity[string, struct{}](100000),
	)
	go rfCache.Start()

	return []LogExporterOption{
		WithLogger(zap.NewNop()),
		WithMeter(noop.NewMeterProvider().Meter(metadata.ScopeName)),
		WithKeysCache(keysCache),
		WithRFCache(rfCache),
	}
}

// setupTestExporter creates a new exporter with mock ClickHouse client for testing
func setupTestExporter(t *testing.T, mock driver.Conn) *clickhouseLogsExporter {
	opts := testOptions()
	opts = append(opts, WithClickHouseClient(mock))
	id := uuid.New()
	opts = append(opts, WithNewUsageCollector(id, mock))

	exporter, err := newExporter(
		exporter.Settings{},
		&Config{
			DSN: "clickhouse://localhost:9000/test",
			AttributesLimits: AttributesLimits{
				FetchKeysInterval: 2 * time.Second,
				MaxDistinctValues: 25000,
			},
		},
		opts...,
	)

	if err != nil {
		t.Fatalf("failed to create exporter: %v", err)
	}

	return exporter
}

func TestExporterInit(t *testing.T) {
	mock, err := cmock.NewClickHouseWithQueryMatcher(nil, sqlmock.QueryMatcherRegexp)
	if err != nil {
		log.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	cols := make([]cmock.ColumnType, 0)
	cols = append(cols, cmock.ColumnType{Name: "tag_key", Type: "String"})
	cols = append(cols, cmock.ColumnType{Name: "tag_type", Type: "String"})
	cols = append(cols, cmock.ColumnType{Name: "tag_data_type", Type: "String"})
	cols = append(cols, cmock.ColumnType{Name: "string_count", Type: "UInt64"})
	cols = append(cols, cmock.ColumnType{Name: "number_count", Type: "UInt64"})

	rows := cmock.NewRows(cols, [][]interface{}{{"key1", "string", "string", 2, 1}, {"key2", "number", "number", 1, 2}})

	mock.ExpectSelect(".*SETTINGS max_threads = 2").WillReturnRows(rows)
	mock.ExpectClose()

	exporter := setupTestExporter(t, mock)
	err = exporter.Start(context.Background(), nil)
	assert.Nil(t, err)

	err = exporter.Shutdown(context.Background())
	assert.Nil(t, err)

	eventually(t, func() bool {
		return mock.ExpectationsWereMet() == nil
	})
}

func TestExporterPushLogsData(t *testing.T) {
	mock, err := cmock.NewClickHouseWithQueryMatcher(nil, sqlmock.QueryMatcherRegexp)
	if err != nil {
		log.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	// expect prepare batch for 5 tables
	tagStatementV2 := mock.ExpectPrepareBatch("INSERT INTO signoz_logs.distributed_tag_attributes_v2")
	attributeKeysStmt := mock.ExpectPrepareBatch("INSERT INTO signoz_logs.distributed_logs_attribute_keys")
	resourceKeysStmt := mock.ExpectPrepareBatch("INSERT INTO signoz_logs.distributed_logs_resource_keys")
	logsStatementV2 := mock.ExpectPrepareBatch("INSERT INTO signoz_logs.distributed_logs_v2.*")
	logsResourceStatementV2 := mock.ExpectPrepareBatch("INSERT INTO signoz_logs.distributed_logs_v2_resource.*")

	tagStatementV2.ExpectAppend()
	tagStatementV2.ExpectSend()
	attributeKeysStmt.ExpectAppend()
	attributeKeysStmt.ExpectSend()
	resourceKeysStmt.ExpectAppend()
	resourceKeysStmt.ExpectSend()
	logsStatementV2.ExpectAppend()
	logsStatementV2.ExpectSend()
	logsResourceStatementV2.ExpectAppend()
	logsResourceStatementV2.ExpectSend()

	// make sure usage is inserted on shutdown
	mock.ExpectExec(".*insert into signoz_logs.distributed_usage.*").WithArgs()
	mock.ExpectClose()

	exporter := setupTestExporter(t, mock)

	logs := plogsgen.Generate()

	err = exporter.pushLogsData(context.Background(), logs)
	if err != nil {
		t.Fatalf("failed to push logs data: %v", err)
	}

	err = exporter.Shutdown(context.Background())
	assert.Nil(t, err)

	eventually(t, func() bool {
		return mock.ExpectationsWereMet() == nil
	})
}

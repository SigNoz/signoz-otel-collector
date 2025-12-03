package clickhouselogsexporter

import (
	"context"
	"log"
	"testing"
	"time"

	driver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/SigNoz/signoz-otel-collector/pkg/pdatagen/plogsgen"
	"github.com/SigNoz/signoz-otel-collector/utils"
	"github.com/google/uuid"
	"github.com/jellydator/ttlcache/v3"
	cmock "github.com/srikanthccv/ClickHouse-go-mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
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
		ttlcache.WithDisableTouchOnHit[string, struct{}](),
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

	cfg := &Config{
		DSN: "clickhouse://localhost:9000/test",
		AttributesLimits: AttributesLimits{
			FetchKeysInterval: 2 * time.Second,
			MaxDistinctValues: 25000,
		},
	}

	require.NoError(t, cfg.Validate())

	exporter, err := newExporter(
		exporter.Settings{},
		cfg,
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

func TestGetResourceAttributesByte(t *testing.T) {
	tests := []struct {
		name      string
		attribute string
		pass      bool
	}{
		{
			name:      "add_signoz_resource_attribute",
			attribute: "signoz.workspace.internal.test",
			pass:      true,
		},
		{
			name:      "add_non_signoz_resource_attribute",
			attribute: "nonsignoz.internal.test",
			pass:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			plogs := plogsgen.Generate(plogsgen.WithResourceAttributeCount(10))

			// get the base expected value
			withoutSigNozAttrs, err := getResourceAttributesByte(plogs.ResourceLogs().At(0).Resource())
			require.NoError(t, err)

			// add the provided attributeand get the actual value
			plogs.ResourceLogs().At(0).Resource().Attributes().PutStr(tc.attribute, "test")
			WithNewAttribute, err := getResourceAttributesByte(plogs.ResourceLogs().At(0).Resource())
			require.NoError(t, err)

			if tc.pass {
				assert.Equal(t, withoutSigNozAttrs, WithNewAttribute)
			} else {
				assert.NotEqual(t, withoutSigNozAttrs, WithNewAttribute)
			}

		})
	}
}

// setupTestExporterWithConcurrency creates a new exporter with mock ClickHouse client and custom concurrency for testing
func setupTestExporterWithConcurrency(t *testing.T, mock driver.Conn, concurrency int) *clickhouseLogsExporter {
	opts := testOptions()
	id := uuid.New()
	opts = append(opts, WithClickHouseClient(mock), WithNewUsageCollector(id, mock), WithConcurrency(concurrency))

	cfg := &Config{
		DSN: "clickhouse://localhost:9000/test",
		AttributesLimits: AttributesLimits{
			FetchKeysInterval: 2 * time.Second,
			MaxDistinctValues: 25000,
		},
	}

	require.NoError(t, cfg.Validate())

	exporter, err := newExporter(
		exporter.Settings{},
		cfg,
		opts...,
	)

	if err != nil {
		t.Fatalf("failed to create exporter: %v", err)
	}

	return exporter
}

func TestConcurrencyCapacity(t *testing.T) {
	exporter := setupTestExporterWithConcurrency(t, nil, 7)
	assert.Equal(t, 7, cap(exporter.limiter))

	eventually(t, func() bool {
		return exporter.Shutdown(context.Background()) == nil
	})
}

func TestExporterConcurrency(t *testing.T) {
	tests := []struct {
		name        string
		logCount    int
		concurrency int
	}{
		{
			name:        "1_log",
			logCount:    1,
			concurrency: 3,
		},
		{
			name:        "7_logs",
			logCount:    7,
			concurrency: 3,
		},
		{
			name:        "10k_logs",
			logCount:    10000,
			concurrency: 3,
		},
		{
			name:        "2234_logs",
			logCount:    2234,
			concurrency: 3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
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
			// Expect appends for logs matching the log count
			for i := 0; i < tc.logCount; i++ {
				logsStatementV2.ExpectAppend()
			}
			logsStatementV2.ExpectSend()
			logsResourceStatementV2.ExpectAppend()
			logsResourceStatementV2.ExpectSend()

			mock.ExpectClose()

			exporter := setupTestExporterWithConcurrency(t, mock, tc.concurrency)
			logs := plogsgen.Generate(plogsgen.WithLogRecordCount(tc.logCount))

			err = exporter.pushLogsData(context.Background(), logs)
			require.NoError(t, err)

			err = exporter.Shutdown(context.Background())
			assert.Nil(t, err)

			eventually(t, func() bool {
				return mock.ExpectationsWereMet() == nil
			})
		})
	}
}

func TestProcessBody(t *testing.T) {
	tests := []struct {
		name                   string
		bodyJSONEnabled        bool
		bodyJSONOldBodyEnabled bool
		promotedPaths          map[string]struct{}
		body                   func() pcommon.Value
		expectedBody           string
		expectedBodyJSON       string
		expectedPromoted       string
	}{
		{
			name:                   "bodyJSONEnabled_false_string_body",
			bodyJSONEnabled:        false,
			bodyJSONOldBodyEnabled: false,
			promotedPaths:          map[string]struct{}{},
			body: func() pcommon.Value {
				v := pcommon.NewValueStr("test log message")
				return v
			},
			expectedBody:     "test log message",
			expectedBodyJSON: "{}",
			expectedPromoted: "{}",
		},
		{
			name:                   "bodyJSONEnabled_false_map_body",
			bodyJSONEnabled:        false,
			bodyJSONOldBodyEnabled: false,
			promotedPaths:          map[string]struct{}{},
			body: func() pcommon.Value {
				v := pcommon.NewValueMap()
				v.Map().PutStr("message", "test")
				return v
			},
			expectedBody:     `{"message":"test"}`,
			expectedBodyJSON: "{}",
			expectedPromoted: "{}",
		},
		{
			name:                   "bodyJSONEnabled_true_non_map_body",
			bodyJSONEnabled:        true,
			bodyJSONOldBodyEnabled: false,
			promotedPaths:          map[string]struct{}{},
			body: func() pcommon.Value {
				v := pcommon.NewValueStr("test log message")
				return v
			},
			expectedBody:     "test log message",
			expectedBodyJSON: "{}",
			expectedPromoted: "{}",
		},
		{
			name:                   "bodyJSONEnabled_true_map_body_with_promoted_paths",
			bodyJSONEnabled:        true,
			bodyJSONOldBodyEnabled: true,
			promotedPaths: map[string]struct{}{
				"message": {},
			},
			body: func() pcommon.Value {
				v := pcommon.NewValueMap()
				v.Map().PutStr("message", "test")
				v.Map().PutInt("level", 1)
				return v
			},
			expectedBody:     `{"level":1,"message":"test"}`,
			expectedBodyJSON: `{"level":1}`,
			expectedPromoted: `{"message":"test"}`,
		},
		{
			name:                   "bodyJSONEnabled_true_map_body_with_nested_promoted_paths",
			bodyJSONEnabled:        true,
			bodyJSONOldBodyEnabled: true,
			promotedPaths: map[string]struct{}{
				"user.id": {},
				"message": {},
			},
			body: func() pcommon.Value {
				v := pcommon.NewValueMap()
				v.Map().PutStr("message", "test")
				userMap := v.Map().PutEmptyMap("user")
				userMap.PutStr("id", "123")
				userMap.PutStr("name", "john")
				return v
			},
			expectedBody:     `{"message":"test","user":{"id":"123","name":"john"}}`,
			expectedBodyJSON: `{"user":{"name":"john"}}`,
			expectedPromoted: `{"message":"test","user.id":"123"}`,
		},
		{
			name:                   "bodyJSONEnabled_true_bodyJSONOldBodyEnabled_true",
			bodyJSONEnabled:        true,
			bodyJSONOldBodyEnabled: false,
			promotedPaths: map[string]struct{}{
				"message": {},
			},
			body: func() pcommon.Value {
				v := pcommon.NewValueMap()
				v.Map().PutStr("message", "test")
				return v
			},
			expectedBody:     "",
			expectedBodyJSON: `{}`,
			expectedPromoted: `{"message":"test"}`,
		},
		{
			name:                   "bodyJSONEnabled_true_map_body_multiple_promoted_paths",
			bodyJSONEnabled:        true,
			bodyJSONOldBodyEnabled: true,
			promotedPaths: map[string]struct{}{
				"level":      {},
				"user.id":    {},
				"user.name":  {},
				"user.roles": {},
				"message":    {},
			},
			body: func() pcommon.Value {
				v := pcommon.NewValueMap()
				v.Map().PutStr("message", "test")
				v.Map().PutInt("level", 1)
				userMap := v.Map().PutEmptyMap("user")
				userMap.PutStr("id", "123")
				userMap.PutStr("name", "john")
				userMap.PutStr("email", "john@example.com")
				roles := userMap.PutEmptySlice("roles")
				roles.AppendEmpty().SetStr("admin")
				roles.AppendEmpty().SetStr("user")
				return v
			},
			expectedBody:     `{"level":1,"message":"test","user":{"email":"john@example.com","id":"123","name":"john","roles":["admin","user"]}}`,
			expectedBodyJSON: `{"user":{"email":"john@example.com"}}`,
			expectedPromoted: `{"level":1,"message":"test","user.id":"123","user.name":"john","user.roles":["admin","user"]}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create exporter with test configuration
			opts := testOptions()
			opts = append(opts, WithClickHouseClient(nil))
			id := uuid.New()
			opts = append(opts, WithNewUsageCollector(id, nil))

			exporter, err := newExporter(
				exporter.Settings{},
				&Config{
					DSN:                       "clickhouse://localhost:9000/test",
					BodyJSONEnabled:           tc.bodyJSONEnabled,
					BodyJSONOldBodyEnabled:    tc.bodyJSONOldBodyEnabled,
					PromotedPathsSyncInterval: utils.ToPointer(5 * time.Minute),
					LogLevelConcurrency:       utils.ToPointer(1),
					AttributesLimits: AttributesLimits{
						FetchKeysInterval: 2 * time.Second,
						MaxDistinctValues: 25000,
					},
				},
				opts...,
			)
			require.NoError(t, err)

			// Set promoted paths
			if tc.promotedPaths != nil {
				exporter.promotedPaths.Store(tc.promotedPaths)
			} else {
				exporter.promotedPaths.Store(map[string]struct{}{})
			}

			// Create body value
			body := tc.body()

			// Process body
			bodyStr, bodyJSONStr, promotedStr := exporter.processBody(body)

			err = exporter.Shutdown(context.Background())
			require.NoError(t, err)

			// Verify results
			assert.Equal(t, tc.expectedBody, bodyStr, "body string mismatch")
			assert.Equal(t, tc.expectedBodyJSON, bodyJSONStr, "bodyJSON string mismatch")
			assert.Equal(t, tc.expectedPromoted, promotedStr, "promoted string mismatch")
		})
	}
}

// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package clickhousetracesexporter

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/SigNoz/signoz-otel-collector/usage"
	"github.com/SigNoz/signoz-otel-collector/utils"
	"github.com/google/uuid"
	"github.com/jellydator/ttlcache/v3"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap"
)

const (
	DISTRIBUTED_TRACES_RESOURCE_V2_SECONDS = 1800
)

type Encoding string

const (
	// EncodingJSON is used for spans encoded as JSON.
	EncodingJSON Encoding = "json"
	// EncodingProto is used for spans encoded as Protobuf.
	EncodingProto Encoding = "protobuf"
)

type shouldSkipKey struct {
	TagKey      string `ch:"tag_key"`
	TagType     string `ch:"tag_type"`
	TagDataType string `ch:"tag_data_type"`
	StringCount uint64 `ch:"string_count"`
	NumberCount uint64 `ch:"number_count"`
}

// SpanWriter for writing spans to ClickHouse
type SpanWriter struct {
	logger            *zap.Logger
	db                clickhouse.Conn
	traceDatabase     string
	indexTable        string
	errorTable        string
	spansTable        string
	attributeTable    string
	attributeTableV2  string
	attributeKeyTable string
	encoding          Encoding
	exporterId        uuid.UUID

	indexTableV3    string
	resourceTableV3 string
	useNewSchema    bool

	keysCache *ttlcache.Cache[string, struct{}]
	rfCache   *ttlcache.Cache[string, struct{}]

	shouldSkipKeyValue atomic.Value // stores map[string]shouldSkipKey

	maxDistinctValues         int
	fetchKeysInterval         time.Duration
	fetchShouldSkipKeysTicker *time.Ticker
}

type WriterOptions struct {
	logger            *zap.Logger
	db                clickhouse.Conn
	traceDatabase     string
	spansTable        string
	indexTable        string
	errorTable        string
	attributeTable    string
	attributeTableV2  string
	attributeKeyTable string
	encoding          Encoding
	exporterId        uuid.UUID

	indexTableV3      string
	resourceTableV3   string
	useNewSchema      bool
	maxDistinctValues int
	fetchKeysInterval time.Duration
}

// NewSpanWriter returns a SpanWriter for the database
func NewSpanWriter(options WriterOptions) *SpanWriter {
	if err := view.Register(SpansCountView, SpansCountBytesView); err != nil {
		return nil
	}

	// keys cache is used to avoid duplicate inserts for the same attribute key.
	keysCache := ttlcache.New[string, struct{}](
		ttlcache.WithTTL[string, struct{}](240*time.Minute),
		ttlcache.WithCapacity[string, struct{}](50000),
	)
	go keysCache.Start()

	// resource fingerprint cache is used to avoid duplicate inserts for the same resource fingerprint.
	// the ttl is set to the same as the bucket rounded value i.e 1800 seconds.
	// if a resource fingerprint is seen in the bucket already, skip inserting it again.
	rfCache := ttlcache.New[string, struct{}](
		ttlcache.WithTTL[string, struct{}](DISTRIBUTED_TRACES_RESOURCE_V2_SECONDS*time.Second),
		ttlcache.WithDisableTouchOnHit[string, struct{}](),
		ttlcache.WithCapacity[string, struct{}](100000),
	)
	go rfCache.Start()

	writer := &SpanWriter{
		logger:            options.logger,
		db:                options.db,
		traceDatabase:     options.traceDatabase,
		indexTable:        options.indexTable,
		errorTable:        options.errorTable,
		spansTable:        options.spansTable,
		attributeTable:    options.attributeTable,
		attributeTableV2:  options.attributeTableV2,
		attributeKeyTable: options.attributeKeyTable,
		encoding:          options.encoding,
		exporterId:        options.exporterId,

		indexTableV3:    options.indexTableV3,
		resourceTableV3: options.resourceTableV3,
		useNewSchema:    options.useNewSchema,
		keysCache:       keysCache,
		rfCache:         rfCache,

		maxDistinctValues:         options.maxDistinctValues,
		fetchKeysInterval:         options.fetchKeysInterval,
		fetchShouldSkipKeysTicker: time.NewTicker(options.fetchKeysInterval),
	}

	go writer.fetchShouldSkipKeys()
	return writer
}

func (e *SpanWriter) fetchShouldSkipKeys() {
	for range e.fetchShouldSkipKeysTicker.C {
		query := fmt.Sprintf(`
			SELECT tag_key, tag_type, tag_data_type, countDistinct(string_value) as string_count, countDistinct(number_value) as number_count
			FROM %s.%s
			GROUP BY tag_key, tag_type, tag_data_type
			HAVING string_count > %d OR number_count > %d
			SETTINGS max_threads = 2`, e.traceDatabase, e.attributeTableV2, e.maxDistinctValues, e.maxDistinctValues)

		e.logger.Info("fetching should skip keys", zap.String("query", query))

		keys := []shouldSkipKey{}

		err := e.db.Select(context.Background(), &keys, query)
		if err != nil {
			e.logger.Error("error while fetching should skip keys", zap.Error(err))
		}

		shouldSkipKeys := make(map[string]shouldSkipKey)
		for _, key := range keys {
			mapKey := utils.MakeKeyForAttributeKeys(key.TagKey, utils.TagType(key.TagType), utils.TagDataType(key.TagDataType))
			e.logger.Debug("adding to should skip keys", zap.String("key", mapKey), zap.Any("string_count", key.StringCount), zap.Any("number_count", key.NumberCount))
			shouldSkipKeys[mapKey] = key
		}
		e.shouldSkipKeyValue.Store(shouldSkipKeys)
	}
}

func (w *SpanWriter) writeIndexBatch(ctx context.Context, batchSpans []*Span) error {
	var statement driver.Batch
	var err error

	defer func() {
		if statement != nil {
			_ = statement.Abort()
		}
	}()
	statement, err = w.db.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s.%s", w.traceDatabase, w.indexTable), driver.WithReleaseConnection())
	if err != nil {
		return fmt.Errorf("could not prepare batch for index table: %w", err)
	}

	for _, span := range batchSpans {
		err = statement.Append(
			time.Unix(0, int64(span.StartTimeUnixNano)),
			span.TraceId,
			span.SpanId,
			span.ParentSpanId,
			span.ServiceName,
			span.Name,
			span.Kind,
			span.DurationNano,
			span.StatusCode,
			span.ExternalHttpMethod,
			span.ExternalHttpUrl,
			span.DBSystem,
			span.DBName,
			span.DBOperation,
			span.PeerService,
			span.Events,
			span.HttpMethod,
			span.HttpUrl,
			span.HttpRoute,
			span.HttpHost,
			span.MsgSystem,
			span.MsgOperation,
			span.HasError,
			span.RPCSystem,
			span.RPCService,
			span.RPCMethod,
			span.ResponseStatusCode,
			span.StringTagMap,
			span.NumberTagMap,
			span.BoolTagMap,
			span.ResourceTagsMap,
			span.IsRemote,
			span.StatusMessage,
			span.StatusCodeString,
			span.SpanKind,
		)
		if err != nil {
			return fmt.Errorf("could not append span to batch: %w", err)
		}
	}

	start := time.Now()

	err = statement.Send()

	ctx, _ = tag.New(ctx,
		tag.Upsert(exporterKey, pipeline.SignalTraces.String()),
		tag.Upsert(tableKey, w.indexTable),
	)
	stats.Record(ctx, writeLatencyMillis.M(int64(time.Since(start).Milliseconds())))
	return err
}

func stringToBool(s string) bool {
	return strings.ToLower(s) == "true"
}

func (w *SpanWriter) writeModelBatch(ctx context.Context, batchSpans []*Span) error {
	var statement driver.Batch
	var err error

	defer func() {
		if statement != nil {
			_ = statement.Abort()
		}
	}()

	statement, err = w.db.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s.%s", w.traceDatabase, w.spansTable), driver.WithReleaseConnection())
	if err != nil {
		return fmt.Errorf("could not prepare batch for model table: %w", err)
	}

	metrics := map[string]usage.Metric{}
	for _, span := range batchSpans {
		var serialized []byte
		usageMap := span.TraceModel
		usageMap.TagMap = map[string]string{}
		serialized, err = json.Marshal(span.TraceModel)
		if err != nil {
			return fmt.Errorf("could not marshal trace model: %w", err)
		}

		serializedUsage, err := json.Marshal(usageMap)
		if err != nil {
			return fmt.Errorf("could not marshal usage map: %w", err)
		}

		err = statement.Append(time.Unix(0, int64(span.StartTimeUnixNano)), span.TraceId, string(serialized))
		if err != nil {
			return fmt.Errorf("could not append span to batch: %w", err)
		}

		if !w.useNewSchema {
			usage.AddMetric(metrics, *span.Tenant, 1, int64(len(serializedUsage)))
		}
	}

	start := time.Now()

	err = statement.Send()
	ctx, _ = tag.New(ctx,
		tag.Upsert(exporterKey, pipeline.SignalTraces.String()),
		tag.Upsert(tableKey, w.spansTable),
	)
	stats.Record(ctx, writeLatencyMillis.M(int64(time.Since(start).Milliseconds())))
	if err != nil {
		return fmt.Errorf("could not send batch to model table: %w", err)
	}

	if !w.useNewSchema {
		for k, v := range metrics {
			stats.RecordWithTags(ctx, []tag.Mutator{tag.Upsert(usage.TagTenantKey, k), tag.Upsert(usage.TagExporterIdKey, w.exporterId.String())}, ExporterSigNozSentSpans.M(int64(v.Count)), ExporterSigNozSentSpansBytes.M(int64(v.Size)))
		}
	}

	return nil
}

// WriteBatchOfSpans writes the encoded batch of spans
func (w *SpanWriter) WriteBatchOfSpans(ctx context.Context, batch []*Span) error {
	// inserts to the singoz_spans table
	if w.spansTable != "" {
		if err := w.writeModelBatch(ctx, batch); err != nil {
			return fmt.Errorf("could not write a batch of spans to model table: %w", err)
		}
	}

	// inserts to the signoz_index_v2 table
	if w.indexTable != "" {
		if err := w.writeIndexBatch(ctx, batch); err != nil {
			return fmt.Errorf("could not write a batch of spans to index table: %w", err)
		}
	}

	return nil
}

// Close closes the writer
func (w *SpanWriter) Close() error {
	if w.db != nil {
		return w.db.Close()
	}
	return nil
}

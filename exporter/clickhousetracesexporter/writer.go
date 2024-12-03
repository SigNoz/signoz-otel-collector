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
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/SigNoz/signoz-otel-collector/usage"
	"github.com/google/uuid"
	"github.com/jellydator/ttlcache/v3"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap"
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

	shouldSkipKey             map[string]shouldSkipKey
	maxDistinctValues         int
	fetchShouldSkipKeysTicker *time.Ticker
	fetchKeysMutex            sync.RWMutex
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

	keysCache := ttlcache.New[string, struct{}](
		ttlcache.WithTTL[string, struct{}](15 * time.Minute),
	)
	rfCache := ttlcache.New[string, struct{}](
		ttlcache.WithTTL[string, struct{}](30*time.Minute),
		ttlcache.WithDisableTouchOnHit[string, struct{}](),
	)

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

		indexTableV3:              options.indexTableV3,
		resourceTableV3:           options.resourceTableV3,
		useNewSchema:              options.useNewSchema,
		shouldSkipKey:             make(map[string]shouldSkipKey),
		maxDistinctValues:         options.maxDistinctValues,
		fetchShouldSkipKeysTicker: time.NewTicker(options.fetchKeysInterval),
		fetchKeysMutex:            sync.RWMutex{},
		keysCache:                 keysCache,
		rfCache:                   rfCache,
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
			HAVING string_count > %d OR number_count > %d`, e.traceDatabase, e.attributeTableV2, e.maxDistinctValues, e.maxDistinctValues)

		e.logger.Info("fetching should skip keys", zap.String("query", query))

		keys := []shouldSkipKey{}

		err := e.db.Select(context.Background(), &keys, query)
		if err != nil {
			e.logger.Error("error while fetching should skip keys", zap.Error(err))
		}

		e.fetchKeysMutex.Lock()
		e.shouldSkipKey = make(map[string]shouldSkipKey)
		for _, key := range keys {
			mapKey := makeKey(key.TagKey, key.TagType, key.TagDataType)
			e.shouldSkipKey[mapKey] = key
		}
		e.fetchKeysMutex.Unlock()
	}
}

func makeKey(tagKey, tagType, tagDataType string) string {
	return fmt.Sprintf("%s:%s:%s", tagKey, tagType, tagDataType)
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
		w.logger.Error("Could not prepare batch for index table: ", zap.Error(err))
		return err
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
			w.logger.Error("Could not append span to batch: ", zap.Object("span", span), zap.Error(err))
			return err
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
		w.logger.Error("Could not prepare batch for model table: ", zap.Error(err))
		return err
	}

	metrics := map[string]usage.Metric{}
	for _, span := range batchSpans {
		var serialized []byte
		usageMap := span.TraceModel
		usageMap.TagMap = map[string]string{}
		serialized, err = json.Marshal(span.TraceModel)
		if err != nil {
			return err
		}
		serializedUsage, err := json.Marshal(usageMap)

		if err != nil {
			return err
		}

		err = statement.Append(time.Unix(0, int64(span.StartTimeUnixNano)), span.TraceId, string(serialized))
		if err != nil {
			w.logger.Error("Could not append span to batch: ", zap.Object("span", span), zap.Error(err))
			return err
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
		return err
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
			w.logger.Error("Could not write a batch of spans to model table: ", zap.Error(err))
			return err
		}
	}

	// inserts to the signoz_index_v2 table
	if w.indexTable != "" {
		if err := w.writeIndexBatch(ctx, batch); err != nil {
			w.logger.Error("Could not write a batch of spans to index table: ", zap.Error(err))
			return err
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

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
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/SigNoz/signoz-otel-collector/utils"
	"github.com/google/uuid"
	"github.com/jellydator/ttlcache/v3"
	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/otel/metric"
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
	durationHistogram metric.Float64Histogram

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
	meter             metric.Meter
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

	durationHistogram, err := options.meter.Float64Histogram(
		"exporter_db_write_latency",
		metric.WithDescription("Time taken to write data to ClickHouse"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(250, 500, 750, 1000, 2000, 2500, 3000, 4000, 5000, 6000, 8000, 10000, 15000, 25000, 30000),
	)
	if err != nil {
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
		durationHistogram: durationHistogram,
		indexTableV3:      options.indexTableV3,
		resourceTableV3:   options.resourceTableV3,
		useNewSchema:      options.useNewSchema,
		keysCache:         keysCache,
		rfCache:           rfCache,

		maxDistinctValues:         options.maxDistinctValues,
		fetchKeysInterval:         options.fetchKeysInterval,
		fetchShouldSkipKeysTicker: time.NewTicker(options.fetchKeysInterval),
	}

	// Fetch keys immediately, then start the background ticker routine
	go func() {
		writer.doFetchShouldSkipKeys() // Immediate first fetch
		writer.fetchShouldSkipKeys()   // Start ticker routine
	}()

	return writer
}

// doFetchShouldSkipKeys contains the logic for fetching skip keys
func (e *SpanWriter) doFetchShouldSkipKeys() {
	query := fmt.Sprintf(`
		SELECT tag_key, tag_type, tag_data_type, countDistinct(string_value) as string_count, countDistinct(number_value) as number_count
		FROM %s.%s
		WHERE unix_milli >= (toUnixTimestamp(now() - toIntervalHour(6)) * 1000)
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

func (e *SpanWriter) fetchShouldSkipKeys() {
	for range e.fetchShouldSkipKeysTicker.C {
		e.doFetchShouldSkipKeys()
	}
}

func stringToBool(s string) bool {
	return strings.ToLower(s) == "true"
}

// Close closes the writer
func (w *SpanWriter) Close() error {
	if w.db != nil {
		return w.db.Close()
	}
	return nil
}

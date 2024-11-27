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
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/SigNoz/signoz-otel-collector/usage"
	"github.com/google/uuid"
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

// SpanWriter for writing spans to ClickHouse
type SpanWriter struct {
	logger            *zap.Logger
	db                clickhouse.Conn
	traceDatabase     string
	indexTable        string
	errorTable        string
	spansTable        string
	attributeTable    string
	attributeKeyTable string
	encoding          Encoding
	exporterId        uuid.UUID

	indexTableV3    string
	resourceTableV3 string
	useNewSchema    bool
}

type WriterOptions struct {
	logger            *zap.Logger
	db                clickhouse.Conn
	traceDatabase     string
	spansTable        string
	indexTable        string
	errorTable        string
	attributeTable    string
	attributeKeyTable string
	encoding          Encoding
	exporterId        uuid.UUID

	indexTableV3    string
	resourceTableV3 string
	useNewSchema    bool
}

// NewSpanWriter returns a SpanWriter for the database
func NewSpanWriter(options WriterOptions) *SpanWriter {
	if err := view.Register(SpansCountView, SpansCountBytesView); err != nil {
		return nil
	}
	writer := &SpanWriter{
		logger:            options.logger,
		db:                options.db,
		traceDatabase:     options.traceDatabase,
		indexTable:        options.indexTable,
		errorTable:        options.errorTable,
		spansTable:        options.spansTable,
		attributeTable:    options.attributeTable,
		attributeKeyTable: options.attributeKeyTable,
		encoding:          options.encoding,
		exporterId:        options.exporterId,

		indexTableV3:    options.indexTableV3,
		resourceTableV3: options.resourceTableV3,
		useNewSchema:    options.useNewSchema,
	}

	return writer
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
			w.logger.Error("could not marshal trace model: ", zap.Error(err))
			w.logger.Error("trace model: ", zap.Any("trace model", span.TraceModel))
			fmt.Printf("trace model: %v\n", span.TraceModel)
			continue
			// return err
		}
		serializedUsage, err := json.Marshal(usageMap)

		if err != nil {
			w.logger.Error("could not marshal usage map: ", zap.Error(err))
			w.logger.Error("usage map: ", zap.Any("usage", usageMap))
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
		w.logger.Error("Could not send batch to model table: ", zap.Error(err))
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

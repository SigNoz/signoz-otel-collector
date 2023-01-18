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
	"math"
	"strings"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/SigNoz/signoz-otel-collector/usage"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/component"
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
	logger        *zap.Logger
	db            clickhouse.Conn
	traceDatabase string
	indexTable    string
	errorTable    string
	spansTable    string
	encoding      Encoding
	delay         time.Duration
	size          int
	spans         chan *Span
	finish        chan bool
	done          sync.WaitGroup
}

// NewSpanWriter returns a SpanWriter for the database
func NewSpanWriter(logger *zap.Logger, db clickhouse.Conn, traceDatabase string, spansTable string, indexTable string, errorTable string, encoding Encoding, delay time.Duration, size int) *SpanWriter {
	if err := view.Register(SpansCountView, SpansCountBytesView); err != nil {
		return nil
	}
	writer := &SpanWriter{
		logger:        logger,
		db:            db,
		traceDatabase: traceDatabase,
		indexTable:    indexTable,
		errorTable:    errorTable,
		spansTable:    spansTable,
		encoding:      encoding,
		delay:         delay,
		size:          size,
		spans:         make(chan *Span, size),
		finish:        make(chan bool),
	}

	go writer.backgroundWriter()

	return writer
}

func (w *SpanWriter) backgroundWriter() {
	batch := make([]*Span, 0, w.size)

	timer := time.After(w.delay)
	last := time.Now()

	for {
		w.done.Add(1)

		flush := false
		finish := false

		select {
		case span := <-w.spans:
			batch = append(batch, span)
			flush = len(batch) == cap(batch)
		case <-timer:
			timer = time.After(w.delay)
			flush = time.Since(last) > w.delay && len(batch) > 0
		case <-w.finish:
			finish = true
			flush = len(batch) > 0
		}

		if flush {
			if err := w.writeBatch(batch); err != nil {
				w.logger.Error("Could not write a batch of spans", zap.Error(err))
			}

			batch = make([]*Span, 0, w.size)
			last = time.Now()
		}

		w.done.Done()

		if finish {
			break
		}
	}
}

func (w *SpanWriter) writeBatch(batch []*Span) error {

	if w.spansTable != "" {
		if err := w.writeModelBatch(batch); err != nil {
			logBatch := batch[:int(math.Min(10, float64(len(batch))))]
			w.logger.Error("Could not write a batch of spans to model table: ", zap.Any("batch", logBatch), zap.Error(err))
			return err
		}
	}
	if w.indexTable != "" {
		if err := w.writeIndexBatch(batch); err != nil {
			logBatch := batch[:int(math.Min(10, float64(len(batch))))]
			w.logger.Error("Could not write a batch of spans to index table: ", zap.Any("batch", logBatch), zap.Error(err))
			return err
		}
	}
	if w.errorTable != "" {
		if err := w.writeErrorBatch(batch); err != nil {
			logBatch := batch[:int(math.Min(10, float64(len(batch))))]
			w.logger.Error("Could not write a batch of spans to error table: ", zap.Any("batch", logBatch), zap.Error(err))
			return err
		}
	}

	return nil
}

func (w *SpanWriter) writeIndexBatch(batchSpans []*Span) error {

	ctx := context.Background()
	statement, err := w.db.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s.%s", w.traceDatabase, w.indexTable))
	if err != nil {
		logBatch := batchSpans[:int(math.Min(10, float64(len(batchSpans))))]
		w.logger.Error("Could not prepare batch for index table: ", zap.Any("batch", logBatch), zap.Error(err))
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
			span.Component,
			span.DBSystem,
			span.DBName,
			span.DBOperation,
			span.PeerService,
			span.Events,
			span.HttpMethod,
			span.HttpUrl,
			span.HttpCode,
			span.HttpRoute,
			span.HttpHost,
			span.MsgSystem,
			span.MsgOperation,
			span.HasError,
			span.TagMap,
			span.GRPCMethod,
			span.GRPCCode,
			span.RPCSystem,
			span.RPCService,
			span.RPCMethod,
			span.ResponseStatusCode,
		)
		if err != nil {
			w.logger.Error("Could not append span to batch: ", zap.Object("span", span), zap.Error(err))
			return err
		}
	}

	start := time.Now()

	err = statement.Send()
	if err != nil {
		return err
	}

	ctx, _ = tag.New(ctx,
		tag.Upsert(exporterKey, string(component.DataTypeMetrics)),
		tag.Upsert(tableKey, w.indexTable),
	)
	stats.Record(ctx, writeLatencyMillis.M(int64(time.Since(start).Milliseconds())))

	return nil
}

func (w *SpanWriter) writeErrorBatch(batchSpans []*Span) error {

	ctx := context.Background()
	statement, err := w.db.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s.%s", w.traceDatabase, w.errorTable))
	if err != nil {
		logBatch := batchSpans[:int(math.Min(10, float64(len(batchSpans))))]
		w.logger.Error("Could not prepare batch for error table: ", zap.Any("batch", logBatch), zap.Error(err))
		return err
	}

	for _, span := range batchSpans {
		if span.ErrorEvent.Name == "" {
			continue
		}
		err = statement.Append(
			time.Unix(0, int64(span.ErrorEvent.TimeUnixNano)),
			span.ErrorID,
			span.ErrorGroupID,
			span.TraceId,
			span.SpanId,
			span.ServiceName,
			span.ErrorEvent.AttributeMap["exception.type"],
			span.ErrorEvent.AttributeMap["exception.message"],
			span.ErrorEvent.AttributeMap["exception.stacktrace"],
			stringToBool(span.ErrorEvent.AttributeMap["exception.escaped"]),
		)
		if err != nil {
			w.logger.Error("Could not append span to batch: ", zap.Object("span", span), zap.Error(err))
			return err
		}
	}

	start := time.Now()

	err = statement.Send()
	if err != nil {
		return err
	}

	ctx, _ = tag.New(ctx,
		tag.Upsert(exporterKey, string(component.DataTypeMetrics)),
		tag.Upsert(tableKey, w.errorTable),
	)
	stats.Record(ctx, writeLatencyMillis.M(int64(time.Since(start).Milliseconds())))

	return nil
}

func stringToBool(s string) bool {
	if strings.ToLower(s) == "true" {
		return true
	}
	return false
}

func (w *SpanWriter) writeModelBatch(batchSpans []*Span) error {
	ctx := context.Background()
	statement, err := w.db.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s.%s", w.traceDatabase, w.spansTable))
	if err != nil {
		logBatch := batchSpans[:int(math.Min(10, float64(len(batchSpans))))]
		w.logger.Error("Could not prepare batch for model table: ", zap.Any("batch", logBatch), zap.Error(err))
		return err
	}

	metrics := map[string]usage.Metric{}
	for _, span := range batchSpans {
		var serialized []byte

		serialized, err = json.Marshal(span.TraceModel)

		if err != nil {
			return err
		}

		err = statement.Append(time.Unix(0, int64(span.StartTimeUnixNano)), span.TraceId, string(serialized))
		if err != nil {
			w.logger.Error("Could not append span to batch: ", zap.Object("span", span), zap.Error(err))
			return err
		}

		usage.AddMetric(metrics, *span.Tenant, 1, int64(len(serialized)))
	}
	start := time.Now()

	err = statement.Send()
	if err != nil {
		return err
	}

	ctx, _ = tag.New(ctx,
		tag.Upsert(exporterKey, string(component.DataTypeMetrics)),
		tag.Upsert(tableKey, w.spansTable),
	)
	stats.Record(ctx, writeLatencyMillis.M(int64(time.Since(start).Milliseconds())))

	for k, v := range metrics {
		stats.RecordWithTags(ctx, []tag.Mutator{tag.Upsert(usage.TagTenantKey, k)}, ExporterSigNozSentSpans.M(int64(v.Count)), ExporterSigNozSentSpansBytes.M(int64(v.Size)))
	}

	return nil
}

// WriteSpan writes the encoded span
func (w *SpanWriter) WriteSpan(span *Span) error {
	w.spans <- span
	return nil
}

// Close Implements io.Closer and closes the underlying storage
func (w *SpanWriter) Close() error {
	w.finish <- true
	w.done.Wait()
	return nil
}

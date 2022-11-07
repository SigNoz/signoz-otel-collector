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
}

// NewSpanWriter returns a SpanWriter for the database
func NewSpanWriter(logger *zap.Logger, db clickhouse.Conn, traceDatabase string, spansTable string, indexTable string, errorTable string, encoding Encoding) *SpanWriter {
	writer := &SpanWriter{
		logger:        logger,
		db:            db,
		traceDatabase: traceDatabase,
		indexTable:    indexTable,
		errorTable:    errorTable,
		spansTable:    spansTable,
		encoding:      encoding,
	}
	return writer
}

func (w *SpanWriter) writeIndexBatch(batchSpans []*Span) error {

	ctx := context.Background()
	statement, err := w.db.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s.%s", w.traceDatabase, w.indexTable))
	if err != nil {
		w.logger.Error("Could not prepare batch for index table: ", zap.Any("batch", batchSpans), zap.Error(err))
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

	return statement.Send()
}

func (w *SpanWriter) writeErrorBatch(batchSpans []*Span) error {

	ctx := context.Background()
	statement, err := w.db.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s.%s", w.traceDatabase, w.errorTable))
	if err != nil {
		w.logger.Error("Could not prepare batch for error table: ", zap.Any("batch", batchSpans), zap.Error(err))
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

	return statement.Send()
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
		w.logger.Error("Could not prepare batch for model table: ", zap.Any("batch", batchSpans), zap.Error(err))
		return err
	}

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
	}

	return statement.Send()
}

// WriteBatchOfSpans writes the encoded batch of spans
func (w *SpanWriter) WriteBatchOfSpans(batch []*Span) error {
	if w.spansTable != "" {
		if err := w.writeModelBatch(batch); err != nil {
			w.logger.Error("Could not write a batch of spans to model table: ", zap.Any("batch", batch), zap.Error(err))
			return err
		}
	}
	if w.indexTable != "" {
		if err := w.writeIndexBatch(batch); err != nil {
			w.logger.Error("Could not write a batch of spans to index table: ", zap.Any("batch", batch), zap.Error(err))
			return err
		}
	}
	if w.errorTable != "" {
		if err := w.writeErrorBatch(batch); err != nil {
			w.logger.Error("Could not write a batch of spans to error table: ", zap.Any("batch", batch), zap.Error(err))
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
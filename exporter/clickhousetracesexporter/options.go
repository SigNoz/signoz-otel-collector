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
	"fmt"
	"time"

	driver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/SigNoz/signoz-otel-collector/usage"
	"github.com/google/uuid"
	"github.com/jellydator/ttlcache/v3"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

type WriterOption func(*SpanWriter)

type TraceExporterOption func(*clickhouseTracesExporter)

func WithClickHouseClient(client driver.Conn) WriterOption {
	return func(e *SpanWriter) {
		e.db = client
	}
}

func WithLogger(logger *zap.Logger) WriterOption {
	return func(e *SpanWriter) {
		e.logger = logger
	}
}

func WithNewUsageCollector(id uuid.UUID, db driver.Conn, logger *zap.Logger) TraceExporterOption {
	return func(e *clickhouseTracesExporter) {
		e.usageCollector = usage.NewUsageCollector(
			id,
			db,
			usage.Options{
				ReportingInterval: usage.DefaultCollectionInterval,
			},
			"signoz_traces",
			UsageExporter,
			logger,
		)
		err := e.usageCollector.Start()
		if err != nil {
			e.logger.Error("Error starting usage collector", zap.Error(err))
		}
		e.id = id
	}
}

func WithMeter(meter metric.Meter) WriterOption {
	return func(e *SpanWriter) {
		durationHistogram, err := meter.Float64Histogram(
			"exporter_db_write_latency",
			metric.WithDescription("Time taken to write data to ClickHouse"),
			metric.WithUnit("ms"),
			metric.WithExplicitBucketBoundaries(250, 500, 750, 1000, 2000, 2500, 3000, 4000, 5000, 6000, 8000, 10000, 15000, 25000, 30000),
		)
		if err != nil {
			panic(fmt.Errorf("error creating duration histogram: %w", err))
		}
		e.durationHistogram = durationHistogram
	}
}

func WithKeysCache(keysCache *ttlcache.Cache[string, struct{}]) WriterOption {
	return func(e *SpanWriter) {
		e.keysCache = keysCache
	}
}

func WithRFCache(rfCache *ttlcache.Cache[string, struct{}]) WriterOption {
	return func(e *SpanWriter) {
		e.rfCache = rfCache
	}
}

func WithAttributesLimits(limits AttributesLimits) WriterOption {
	return func(e *SpanWriter) {
		e.maxDistinctValues = limits.MaxDistinctValues
		e.fetchShouldSkipKeysTicker = time.NewTicker(limits.FetchKeysInterval)
	}
}

func WithExporterID(id uuid.UUID) WriterOption {
	return func(e *SpanWriter) {
		e.exporterId = id
	}
}

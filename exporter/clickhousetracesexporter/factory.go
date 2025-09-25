// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
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
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
	"github.com/jellydator/ttlcache/v3"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/SigNoz/signoz-otel-collector/exporter/clickhousetracesexporter/internal/metadata"
)

func createDefaultConfig() component.Config {
	return &Config{
		TimeoutConfig:    exporterhelper.NewDefaultTimeoutConfig(),
		QueueBatchConfig: exporterhelper.NewDefaultQueueConfig(),
		BackOffConfig:    configretry.NewDefaultBackOffConfig(),
		AttributesLimits: AttributesLimits{
			FetchKeysInterval: 10 * time.Minute,
			MaxDistinctValues: 25000,
		},
	}
}

// NewFactory creates a factory for Logging exporter
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, metadata.TracesStability),
	)
}

func createTracesExporter(
	ctx context.Context,
	params exporter.Settings,
	cfg component.Config,
) (exporter.Traces, error) {

	c := cfg.(*Config)

	client, err := newClickhouseClient(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("failed to create clickhouse client: %w", err)
	}

	id := uuid.New()

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
		ttlcache.WithTTL[string, struct{}](distributedTracesResourceV2Seconds*time.Second),
		ttlcache.WithDisableTouchOnHit[string, struct{}](),
		ttlcache.WithCapacity[string, struct{}](100000),
	)
	go rfCache.Start()

	meter := params.MeterProvider.Meter(metadata.ScopeName)
	writerOpts := []WriterOption{
		WithClickHouseClient(client),
		WithLogger(params.Logger),
		WithMeter(meter),
		WithKeysCache(keysCache),
		WithRFCache(rfCache),
		WithAttributesLimits(c.AttributesLimits),
		WithExporterID(id),
	}

	exporterOpts := []TraceExporterOption{
		WithNewUsageCollector(id, client, params.Logger),
	}

	exporter, err := newExporter(c, params, writerOpts, exporterOpts)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewTraces(
		ctx,
		params,
		cfg,
		exporter.pushTraceData,
		exporterhelper.WithShutdown(exporter.Shutdown),
		exporterhelper.WithTimeout(c.TimeoutConfig),
		exporterhelper.WithQueue(c.QueueBatchConfig),
		exporterhelper.WithRetry(c.BackOffConfig))
}

func newClickhouseClient(ctx context.Context, cfg *Config) (clickhouse.Conn, error) {
	options, err := clickhouse.ParseDSN(cfg.Datasource)
	if err != nil {
		return nil, err
	}
	// setting maxIdleConnections = numConsumers + 1 to avoid `prepareBatch:clickhouse: acquire conn timeout` error
	maxIdleConnections := cfg.QueueBatchConfig.NumConsumers + 1
	if options.MaxIdleConns < maxIdleConnections {
		options.MaxIdleConns = maxIdleConnections
		options.MaxOpenConns = maxIdleConnections + 5
	}
	db, err := clickhouse.Open(options)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(ctx); err != nil {
		return nil, err
	}
	return db, nil
}

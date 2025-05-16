// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package clickhouselogsexporter

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jellydator/ttlcache/v3"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr          = "clickhouselogsexporter"
	primaryNamespace = "clickhouselogs"
	archiveNamespace = "clickhouselogs-archive"
	databaseName     = "signoz_logs"
)

// NewFactory creates a factory for Elastic exporter.
func NewFactory() exporter.Factory {

	return exporter.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		exporter.WithLogs(createLogsExporter, component.StabilityLevelBeta),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		TimeoutConfig: exporterhelper.NewDefaultTimeoutConfig(),
		QueueConfig:   exporterhelper.NewDefaultQueueConfig(),
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
		AttributesLimits: AttributesLimits{
			FetchKeysInterval: 10 * time.Minute,
			MaxDistinctValues: 25000,
		},
	}
}

// createLogsExporter creates a new exporter for logs.
// Logs are directly insert into clickhouse.
func createLogsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Logs, error) {
	c := cfg.(*Config)

	client, err := newClickhouseClient(set.Logger, c)
	if err != nil {
		return nil, fmt.Errorf("cannot configure clickhouse logs exporter: %w", err)
	}

	id := uuid.New()

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
		ttlcache.WithTTL[string, struct{}](distributedLogsResourceV2Seconds*time.Second),
		ttlcache.WithDisableTouchOnHit[string, struct{}](),
		ttlcache.WithCapacity[string, struct{}](100000),
	)
	go rfCache.Start()

	meter := set.MeterProvider.Meter("github.com/SigNoz/signoz-otel-collector/exporter/clickhouselogsexporter")
	opts := []LogExporterOption{
		WithClickHouseClient(client),
		WithLogger(set.Logger),
		WithNewUsageCollector(id, client),
		WithMeter(meter),
		WithKeysCache(keysCache),
		WithRFCache(rfCache),
	}

	exporter, err := newExporter(set, c, opts...)
	if err != nil {
		return nil, fmt.Errorf("cannot configure clickhouse logs exporter: %w", err)
	}

	return exporterhelper.NewLogs(
		ctx,
		set,
		cfg,
		exporter.pushLogsData,
		exporterhelper.WithStart(exporter.Start),
		exporterhelper.WithShutdown(exporter.Shutdown),
		exporterhelper.WithTimeout(c.TimeoutConfig),
		exporterhelper.WithQueue(c.QueueConfig),
		exporterhelper.WithRetry(c.BackOffConfig),
	)
}

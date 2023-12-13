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

package clickhousemetricsexporter

import (
	"context"
	"errors"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr = "clickhousemetricswrite"
)

var (
	writeLatencyMillis = stats.Int64("exporter_db_write_latency", "Time taken (in millis) for exporter to write batch", "ms")
	exporterKey        = tag.MustNewKey("exporter")
	tableKey           = tag.MustNewKey("table")
)

// NewFactory creates a new Prometheus Remote Write exporter.
func NewFactory() exporter.Factory {

	writeLatencyDistribution := view.Distribution(100, 250, 500, 750, 1000, 2000, 4000, 8000, 16000, 32000, 64000, 128000, 256000, 512000)

	writeLatencyView := &view.View{
		Name:        "exporter_db_write_latency",
		Measure:     writeLatencyMillis,
		Description: writeLatencyMillis.Description(),
		TagKeys:     []tag.Key{exporterKey, tableKey},
		Aggregation: writeLatencyDistribution,
	}

	view.Register(writeLatencyView)
	return exporter.NewFactory(
		typeStr,
		createDefaultConfig,
		exporter.WithMetrics(createMetricsExporter, component.StabilityLevelUndefined))
}

func createMetricsExporter(ctx context.Context, set exporter.CreateSettings,
	cfg component.Config) (exporter.Metrics, error) {

	cheCfg, ok := cfg.(*Config)
	if !ok {
		return nil, errors.New("invalid configuration")
	}

	if cheCfg.Cluster == "" {
		cheCfg.Cluster = "cluster" // default cluster name
	}

	che, err := NewClickHouseExporter(cheCfg, set)
	if err != nil {
		return nil, err
	}

	exporter, err := exporterhelper.NewMetricsExporter(
		ctx,
		set,
		cfg,
		che.PushMetrics,
		exporterhelper.WithTimeout(cheCfg.TimeoutSettings),
		exporterhelper.WithQueue(cheCfg.QueueSettings),
		exporterhelper.WithRetry(cheCfg.RetrySettings),
		exporterhelper.WithStart(che.Start),
		exporterhelper.WithShutdown(che.Shutdown),
	)

	if err != nil {
		return nil, err
	}

	return resourcetotelemetry.WrapMetricsExporter(cheCfg.ResourceToTelemetrySettings, exporter), nil
}

func createDefaultConfig() component.Config {
	return &Config{
		TimeoutSettings: exporterhelper.NewDefaultTimeoutSettings(),
		RetrySettings: exporterhelper.RetrySettings{
			Enabled:         true,
			InitialInterval: 50 * time.Millisecond,
			MaxInterval:     200 * time.Millisecond,
			MaxElapsedTime:  1 * time.Minute,
		},
		QueueSettings:   exporterhelper.NewDefaultQueueSettings(),
		WatcherInterval: 30 * time.Second,
		WriteTSToV4:     false,
	}
}

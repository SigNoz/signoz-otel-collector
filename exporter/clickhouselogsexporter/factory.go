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

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr          = "clickhouselogsexporter"
	primaryNamespace = "clickhouselogs"
	archiveNamespace = "clickhouselogs-archive"
	databaseName     = "signoz_logs"
	tableName        = "logs"
	migrationsFolder = "./migrations"
)

// NewFactory creates a factory for Elastic exporter.
func NewFactory() component.ExporterFactory {
	return component.NewExporterFactory(
		typeStr,
		createDefaultConfig,
		component.WithLogsExporter(createLogsExporter, component.StabilityLevelBeta),
	)
}

func createDefaultConfig() config.Exporter {
	return &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
		TimeoutSettings:  exporterhelper.NewDefaultTimeoutSettings(),
		QueueSettings:    QueueSettings{QueueSize: exporterhelper.NewDefaultQueueSettings().QueueSize},
		RetrySettings:    exporterhelper.NewDefaultRetrySettings(),
	}
}

// createLogsExporter creates a new exporter for logs.
// Logs are directly insert into clickhouse.
func createLogsExporter(
	ctx context.Context,
	set component.ExporterCreateSettings,
	cfg config.Exporter,
) (component.LogsExporter, error) {
	c := cfg.(*Config)
	exporter, err := newExporter(set.Logger, c)
	if err != nil {
		return nil, fmt.Errorf("cannot configure clickhouse logs exporter: %w", err)
	}

	return exporterhelper.NewLogsExporter(
		ctx,
		set,
		cfg,
		exporter.pushLogsData,
		exporterhelper.WithShutdown(exporter.Shutdown),
		exporterhelper.WithTimeout(c.TimeoutSettings),
		exporterhelper.WithQueue(c.enforcedQueueSettings()),
		exporterhelper.WithRetry(c.RetrySettings),
	)
}

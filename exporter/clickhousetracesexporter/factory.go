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
	"io"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr          = "clickhousetraces"
	primaryNamespace = "clickhouse"
	archiveNamespace = "clickhouse-archive"
)

func createDefaultConfig() config.Exporter {
	// opts := NewOptions(primaryNamespace, archiveNamespace)
	return &Config{
		// Options:          *opts,
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
	}
}

// NewFactory creates a factory for Logging exporter
func NewFactory() component.ExporterFactory {
	return exporterhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		exporterhelper.WithTraces(createTracesExporter),
	)
}

func createTracesExporter(
	_ context.Context,
	params component.ExporterCreateSettings,
	cfg config.Exporter,
) (component.TracesExporter, error) {

	oce, err := newExporter(cfg, params.Logger)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewTracesExporter(
		cfg,
		params,
		oce.pushTraceData,
		exporterhelper.WithShutdown(func(context.Context) error {
			if closer, ok := oce.Writer.(io.Closer); ok {
				return closer.Close()
			}
			return nil
		}))
}

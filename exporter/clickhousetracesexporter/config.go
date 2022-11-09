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
	"fmt"

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config defines configuration for logging exporter.
type Config struct {
	config.ExporterSettings        `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct
	exporterhelper.TimeoutSettings `mapstructure:",squash"`
	exporterhelper.RetrySettings   `mapstructure:"retry_on_failure"`
	exporterhelper.QueueSettings   `mapstructure:"sending_queue"`
	Options                        `mapstructure:",squash"`
	Datasource                     string `mapstructure:"datasource"`
	Migrations                     string `mapstructure:"migrations"`
}

var _ config.Exporter = (*Config)(nil)

// Validate checks if the exporter configuration is valid
func (cfg *Config) Validate() error {
	if cfg.QueueSettings.QueueSize < 0 {
		return fmt.Errorf("remote write queue size can't be negative")
	}

	if cfg.QueueSettings.Enabled && cfg.QueueSettings.QueueSize == 0 {
		return fmt.Errorf("a 0 size queue will drop all the data")
	}

	if cfg.QueueSettings.NumConsumers < 0 {
		return fmt.Errorf("remote write consumer number can't be negative")
	}
	return nil
}

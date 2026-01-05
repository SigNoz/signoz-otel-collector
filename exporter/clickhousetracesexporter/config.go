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
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

type AttributesLimits struct {
	FetchKeysInterval time.Duration `mapstructure:"fetch_keys_interval"`
	MaxDistinctValues int           `mapstructure:"max_distinct_values"`
}

// Config defines configuration for tracing exporter.
type Config struct {
	exporterhelper.TimeoutConfig `mapstructure:",squash"`
	BackOffConfig                configretry.BackOffConfig                                `mapstructure:"retry_on_failure"`
	QueueBatchConfig             configoptional.Optional[exporterhelper.QueueBatchConfig] `mapstructure:"sending_queue"`

	Datasource string `mapstructure:"datasource"`
	// LowCardinalExceptionGrouping is a flag to enable exception grouping by serviceName + exceptionType. Default is false.
	LowCardinalExceptionGrouping bool `mapstructure:"low_cardinal_exception_grouping"`
	UseNewSchema                 bool `mapstructure:"use_new_schema"`

	AttributesLimits AttributesLimits `mapstructure:"attributes_limits"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the exporter configuration is valid
func (cfg *Config) Validate() error {
	q := cfg.QueueBatchConfig.Get()
	if q != nil {
		if q.QueueSize <= 0 {
			return fmt.Errorf("queue_size must be positive")
		}
		if q.NumConsumers <= 0 {
			return fmt.Errorf("num_consumers must be positive")
		}
	}
	return nil
}

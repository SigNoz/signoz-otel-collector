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
	"errors"
	"time"

	"github.com/SigNoz/signoz-otel-collector/utils"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/multierr"
)

const (
	defaultPromotedPathsSyncInterval = 5 * time.Minute
)

type AttributesLimits struct {
	FetchKeysInterval time.Duration `mapstructure:"fetch_keys_interval"`
	MaxDistinctValues int           `mapstructure:"max_distinct_values"`
}

// Config defines configuration for ClickHouse exporter.
type Config struct {
	exporterhelper.TimeoutConfig `mapstructure:",squash"`
	BackOffConfig                configretry.BackOffConfig       `mapstructure:"retry_on_failure"`
	QueueBatchConfig             exporterhelper.QueueBatchConfig `mapstructure:"sending_queue"`

	// DSN is the ClickHouse server Data Source Name.
	// For tcp protocol reference: [ClickHouse/clickhouse-go#dsn](https://github.com/ClickHouse/clickhouse-go#dsn).
	DSN                 string `mapstructure:"dsn"`
	UseNewSchema        bool   `mapstructure:"use_new_schema"`
	LogLevelConcurrency *int   `mapstructure:"log_level_concurrency"`

	AttributesLimits          AttributesLimits `mapstructure:"attributes_limits"`
	PromotedPathsSyncInterval *time.Duration   `mapstructure:"promoted_paths_sync_interval"`
	BodyJSONEnabled           bool             `mapstructure:"body_json_enabled"`
	BodyJSONOldBodyEnabled    bool             `mapstructure:"body_json_old_body_enabled"`
}

var (
	errConfigNoDSN = errors.New("dsn must be specified")
)

// Validate validates the clickhouse server configuration.
func (cfg *Config) Validate() (err error) {
	if cfg.DSN == "" {
		err = multierr.Append(err, errConfigNoDSN)
	}
	if cfg.PromotedPathsSyncInterval == nil {
		cfg.PromotedPathsSyncInterval = utils.ToPointer(defaultPromotedPathsSyncInterval)
	}

	if cfg.LogLevelConcurrency == nil {
		cfg.LogLevelConcurrency = utils.ToPointer(utils.Concurrency())
	}
	if *cfg.LogLevelConcurrency < 1 {
		err = multierr.Append(err, errors.New("concurrency must be greater than 0"))
	}

	return err
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadataexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/metadataexporter"

import (
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

type TracesMaxDistinctValuesConfig struct {
	MaxKeys                  uint64 `mapstructure:"max_keys"`
	MaxStringDistinctValues  uint64 `mapstructure:"max_string_distinct_values"`
	MaxStringLength          uint64 `mapstructure:"max_string_length"`
	MaxInt64DistinctValues   uint64 `mapstructure:"max_int64_distinct_values"`
	MaxFloat64DistinctValues uint64 `mapstructure:"max_float64_distinct_values"`
}

type LogsMaxDistinctValuesConfig struct {
	MaxKeys                  uint64 `mapstructure:"max_keys"`
	MaxStringDistinctValues  uint64 `mapstructure:"max_string_distinct_values"`
	MaxStringLength          uint64 `mapstructure:"max_string_length"`
	MaxInt64DistinctValues   uint64 `mapstructure:"max_int64_distinct_values"`
	MaxFloat64DistinctValues uint64 `mapstructure:"max_float64_distinct_values"`
}

type MetricsMaxDistinctValuesConfig struct {
	MaxKeys                  uint64 `mapstructure:"max_keys"`
	MaxStringDistinctValues  uint64 `mapstructure:"max_string_distinct_values"`
	MaxStringLength          uint64 `mapstructure:"max_string_length"`
	MaxInt64DistinctValues   uint64 `mapstructure:"max_int64_distinct_values"`
	MaxFloat64DistinctValues uint64 `mapstructure:"max_float64_distinct_values"`
}

type MaxDistinctValuesConfig struct {
	Traces  TracesMaxDistinctValuesConfig  `mapstructure:"traces"`
	Logs    LogsMaxDistinctValuesConfig    `mapstructure:"logs"`
	Metrics MetricsMaxDistinctValuesConfig `mapstructure:"metrics"`
}

type AlwaysIncludeAttributesConfig struct {
	Traces  []string `mapstructure:"traces"`
	Logs    []string `mapstructure:"logs"`
	Metrics []string `mapstructure:"metrics"`
}

// Config defines configuration for Metadata exporter.
type Config struct {
	exporterhelper.TimeoutConfig `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	exporterhelper.QueueConfig   `mapstructure:"sending_queue"`
	configretry.BackOffConfig    `mapstructure:"retry_on_failure"`

	DSN string `mapstructure:"dsn"`

	MaxDistinctValues MaxDistinctValuesConfig `mapstructure:"max_distinct_values"`

	AlwaysIncludeAttributes AlwaysIncludeAttributesConfig `mapstructure:"always_include_attributes"`
}

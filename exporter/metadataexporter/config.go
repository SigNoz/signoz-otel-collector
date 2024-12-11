package metadataexporter

import (
	"time"

	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

type LimitsConfig struct {
	MaxKeys                 uint64        `mapstructure:"max_keys"`
	MaxStringDistinctValues uint64        `mapstructure:"max_string_distinct_values"`
	MaxStringLength         uint64        `mapstructure:"max_string_length"`
	FetchInterval           time.Duration `mapstructure:"fetch_interval"`
}

type MaxDistinctValuesConfig struct {
	Traces  LimitsConfig `mapstructure:"traces"`
	Logs    LimitsConfig `mapstructure:"logs"`
	Metrics LimitsConfig `mapstructure:"metrics"`
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

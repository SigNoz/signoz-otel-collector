package metadataexporter

import (
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

type CacheProvider string

const (
	CacheProviderInMemory CacheProvider = "in_memory"
	CacheProviderRedis    CacheProvider = "redis"
)

type LimitsConfig struct {
	MaxKeys                 uint64 `mapstructure:"max_keys"`
	MaxStringDistinctValues uint64 `mapstructure:"max_string_distinct_values"`
	// MaxStringLength is the longest attribute value that is written to the
	// metadata table. A key whose value exceeds it is dropped from every
	// subsequent row of the signal until the key has been absent for a while.
	MaxStringLength uint64 `mapstructure:"max_string_length"`
	// MaxResourceStringLength is the same limit applied to resource attribute
	// values. Keys listed in always_include_attributes are exempt.
	MaxResourceStringLength uint64        `mapstructure:"max_resource_string_length"`
	FetchInterval           time.Duration `mapstructure:"fetch_interval"`
	// Bucket is the time window a resource+attribute set is written once per.
	// The query side must floor its window start to the largest bucket in use.
	Bucket time.Duration `mapstructure:"bucket"`
}

func (c LimitsConfig) validate(signal string) error {
	if c.Bucket < time.Millisecond {
		return fmt.Errorf("max_distinct_values::%s::bucket must be at least 1ms", signal)
	}
	if c.MaxStringLength == 0 {
		return fmt.Errorf("max_distinct_values::%s::max_string_length must be positive", signal)
	}
	if c.MaxResourceStringLength == 0 {
		return fmt.Errorf("max_distinct_values::%s::max_resource_string_length must be positive", signal)
	}
	return nil
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

type InMemoryCacheConfig struct {
}

type RedisCacheConfig struct {
	Addr     string `mapstructure:"addr"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type CacheLimits struct {
	MaxResources              uint64 `mapstructure:"max_resources"`
	MaxCardinalityPerResource uint64 `mapstructure:"max_cardinality_per_resource"`
	MaxTotalCardinality       uint64 `mapstructure:"max_total_cardinality"`
}

type CacheConfig struct {
	Provider CacheProvider       `mapstructure:"provider"`
	InMemory InMemoryCacheConfig `mapstructure:"in_memory"`
	Redis    RedisCacheConfig    `mapstructure:"redis"`
	Traces   CacheLimits         `mapstructure:"traces_limits"`
	Metrics  CacheLimits         `mapstructure:"metrics_limits"`
	Logs     CacheLimits         `mapstructure:"logs_limits"`
	// Iterate over all the keys in the cache and print the cardinality
	// Since this is expensive, it is disabled by default
	Debug bool `mapstructure:"debug"`
}

const (
	defaultJSONMaxDepthTraverse        = 22
	defaultJSONMaxArrayElementsAllowed = 100
	defaultJSONMaxKeysAtLevel          = 1024
	defaultJSONKeyCacheSize            = 10_000
)

// JSONConfig holds configuration for JSON field processing (body).
type JSONConfig struct {
	// Enabled gates all JSON field processing (type collection + value suggestions).
	Enabled bool `mapstructure:"enabled"`
	// MaxDepthTraverse is the maximum nesting depth to traverse.
	MaxDepthTraverse *int `mapstructure:"max_depth_traverse"`
	// MaxArrayElementsAllowed is the maximum number of array elements to inspect.
	MaxArrayElementsAllowed *int `mapstructure:"max_array_elements_allowed"`
	// MaxKeysAtLevel is the maximum number of keys allowed at any single map level.
	MaxKeysAtLevel *int `mapstructure:"max_keys_at_level"`
}

// Config defines configuration for Metadata exporter.
type Config struct {
	exporterhelper.TimeoutConfig `mapstructure:",squash"`                                 // squash ensures fields are correctly decoded in embedded struct.
	QueueBatchConfig             configoptional.Optional[exporterhelper.QueueBatchConfig] `mapstructure:"sending_queue"`
	BackOffConfig                configretry.BackOffConfig                                `mapstructure:"retry_on_failure"`

	DSN string `mapstructure:"dsn"`

	MaxDistinctValues MaxDistinctValuesConfig `mapstructure:"max_distinct_values"`

	AlwaysIncludeAttributes AlwaysIncludeAttributesConfig `mapstructure:"always_include_attributes"`

	Cache CacheConfig `mapstructure:"cache"`

	TenantID string `mapstructure:"tenant_id"`

	Enabled bool `mapstructure:"enabled"`

	// JSON configures JSON field processing for body (and attributes in future).
	JSON JSONConfig `mapstructure:"json"`
}

// Validate checks the per-signal limits.
func (cfg *Config) Validate() error {
	return errors.Join(
		cfg.MaxDistinctValues.Traces.validate("traces"),
		cfg.MaxDistinctValues.Logs.validate("logs"),
		cfg.MaxDistinctValues.Metrics.validate("metrics"),
	)
}

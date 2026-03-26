package metadataexporter

import (
	"errors"
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
	MaxDepthTraverse int `mapstructure:"max_depth_traverse"`
	// MaxArrayElementsAllowed is the maximum number of array elements to inspect.
	MaxArrayElementsAllowed int `mapstructure:"max_array_elements_allowed"`
	// MaxKeysAtLevel is the maximum number of keys allowed at any single map level.
	MaxKeysAtLevel int `mapstructure:"max_keys_at_level"`
}

func (c *JSONConfig) Validate() error {
	if c.MaxDepthTraverse <= 0 {
		c.MaxDepthTraverse = defaultJSONMaxDepthTraverse
	}
	if c.MaxArrayElementsAllowed <= 0 {
		c.MaxArrayElementsAllowed = defaultJSONMaxArrayElementsAllowed
	}
	if c.MaxKeysAtLevel <= 0 {
		c.MaxKeysAtLevel = defaultJSONMaxKeysAtLevel
	}
	return nil
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

func (c *Config) Validate() error {
	errs := []error{}
	errs = append(errs, c.TimeoutConfig.Validate())
	errs = append(errs, c.BackOffConfig.Validate())
	errs = append(errs, c.QueueBatchConfig.Validate())
	errs = append(errs, c.JSON.Validate())

	return errors.Join(errs...)
}

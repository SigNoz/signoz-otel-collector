package fieldvalues

import (
	"errors"
	"time"
)

// CacheProvider names the key cache that is shared by the collectors of a
// tenant. The local key cache is always used; a shared provider adds a second
// level that removes repeat inserts across collectors.
type CacheProvider string

const (
	CacheProviderInMemory CacheProvider = "in_memory"
	CacheProviderRedis    CacheProvider = "redis"
)

// LimitsConfig holds the limits for logs and traces. Metrics have no limits.
type LimitsConfig struct {
	// MaxRecordFieldValues is the number of distinct values per day above which
	// a record field leaves the set hash. Each collector still writes up to one
	// more value per day (the sample).
	MaxRecordFieldValues uint64 `mapstructure:"max_record_field_values"`
	// MaxResourceFieldValues is the number of distinct values per day above
	// which a resource field leaves the resource identity. Its values are still
	// written.
	MaxResourceFieldValues uint64 `mapstructure:"max_resource_field_values"`
	// MaxValueBytes is the longest value that is stored.
	MaxValueBytes int `mapstructure:"max_value_bytes"`
	// MaxFieldsPerSignal is the number of field places per day on one
	// collector.
	MaxFieldsPerSignal int `mapstructure:"max_fields_per_signal"`
	// MaxSetsPerResource is the number of new sets per resource per window on
	// one collector, for each coarse step.
	MaxSetsPerResource int `mapstructure:"max_sets_per_resource"`
	// MaxOutsidePairsPerResource is the number of new keys of pairs outside the
	// hash per resource per window on one collector, for each coarse step.
	MaxOutsidePairsPerResource int `mapstructure:"max_outside_pairs_per_resource"`
}

// CacheConfig sizes the local key cache and selects the shared one.
type CacheConfig struct {
	Provider CacheProvider `mapstructure:"provider"`
	// MaxBytes is the memory of one writer (one signal of one exporter): three
	// quarters for the key cache, at least 64 MiB outside the Go heap, and one
	// quarter for the value tracker and the resource states. With 0, the
	// writers of the process share 10% of the Go memory limit, between 64 MiB
	// and 1 GiB, or 256 MiB without a limit.
	MaxBytes uint64 `mapstructure:"max_bytes"`
	// ReserveShare is the share of the local cache kept for overflow sets and
	// metric labels.
	ReserveShare float64 `mapstructure:"reserve_share"`
	// Window is the time in which each set, pair and resource is written once
	// per collector. It must divide a UTC day.
	Window time.Duration `mapstructure:"window"`
	// PreWriteWindow is the last part of each window in which the keys seen
	// are also written for the next window, each at a time set by its hash.
	// It spreads the writes of a new window over this time. The daily sample
	// of high-cardinality fields is spread over the first PreWriteWindow of
	// each UTC day. 0 disables both.
	PreWriteWindow time.Duration `mapstructure:"pre_write_window"`
}

// ClassificationConfig controls the reads of field_values_daily.
type ClassificationConfig struct {
	RefreshInterval time.Duration `mapstructure:"refresh_interval"`
	// LookbackDays is the number of days, today included, on which a field
	// over the limit stays out of the hash.
	LookbackDays int `mapstructure:"lookback_days"`
}

// Config is the field_values section of the metadata exporter.
type Config struct {
	Enabled bool `mapstructure:"enabled"`
	// Source is the space inside the signal, such as "meter" for metrics. It
	// is empty for the default space.
	Source string `mapstructure:"source"`

	Limits         LimitsConfig         `mapstructure:"limits"`
	Cache          CacheConfig          `mapstructure:"cache"`
	Classification ClassificationConfig `mapstructure:"classification"`

	// AlwaysInclude names fields that always stay in the hash, with no limits.
	AlwaysInclude []string `mapstructure:"always_include"`
}

func DefaultConfig() Config {
	return Config{
		Limits: LimitsConfig{
			MaxRecordFieldValues:       5000,
			MaxResourceFieldValues:     100000,
			MaxValueBytes:              256,
			MaxFieldsPerSignal:         4096,
			MaxSetsPerResource:         16384,
			MaxOutsidePairsPerResource: 16384,
		},
		Cache: CacheConfig{
			Provider:       CacheProviderInMemory,
			ReserveShare:   0.1,
			Window:         24 * time.Hour,
			PreWriteWindow: time.Hour,
		},
		Classification: ClassificationConfig{
			RefreshInterval: 15 * time.Minute,
			LookbackDays:    7,
		},
	}
}

func (c *Config) Validate() error {
	var errs []error
	if c.Limits.MaxRecordFieldValues == 0 || c.Limits.MaxResourceFieldValues == 0 {
		errs = append(errs, errors.New("field_values.limits: field value limits must be positive"))
	}
	if c.Limits.MaxValueBytes <= 0 || c.Limits.MaxFieldsPerSignal <= 0 {
		errs = append(errs, errors.New("field_values.limits: max_value_bytes and max_fields_per_signal must be positive"))
	}
	if c.Limits.MaxSetsPerResource < 2 || c.Limits.MaxOutsidePairsPerResource < 1 {
		errs = append(errs, errors.New("field_values.limits: max_sets_per_resource must be at least 2 and max_outside_pairs_per_resource at least 1"))
	}
	if c.Cache.Provider != CacheProviderInMemory && c.Cache.Provider != CacheProviderRedis {
		errs = append(errs, errors.New("field_values.cache: provider must be in_memory or redis"))
	}
	if c.Cache.ReserveShare <= 0 || c.Cache.ReserveShare >= 1 {
		errs = append(errs, errors.New("field_values.cache: reserve_share must be between 0 and 1"))
	}
	if c.Cache.Window < time.Minute || (24*time.Hour)%c.Cache.Window != 0 {
		errs = append(errs, errors.New("field_values.cache: window must be at least 1m and divide 24h"))
	}
	if c.Cache.PreWriteWindow < 0 || c.Cache.PreWriteWindow >= c.Cache.Window || (c.Cache.PreWriteWindow > 0 && c.Cache.PreWriteWindow < time.Second) {
		errs = append(errs, errors.New("field_values.cache: pre_write_window must be 0, or at least 1s and shorter than window"))
	}
	if c.Classification.RefreshInterval <= 0 {
		errs = append(errs, errors.New("field_values.classification: refresh_interval must be positive"))
	}
	if c.Classification.LookbackDays < 1 || c.Classification.LookbackDays > 30 {
		errs = append(errs, errors.New("field_values.classification: lookback_days must be between 1 and 30"))
	}
	return errors.Join(errs...)
}

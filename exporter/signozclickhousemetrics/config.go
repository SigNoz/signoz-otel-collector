package signozclickhousemetrics

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config defines configuration for ClickHouse Metrics exporter.
type Config struct {
	exporterhelper.TimeoutConfig `mapstructure:",squash"`                                 // squash ensures fields are correctly decoded in embedded struct.
	BackOffConfig                configretry.BackOffConfig                                `mapstructure:"retry_on_failure"`
	QueueBatchConfig             configoptional.Optional[exporterhelper.QueueBatchConfig] `mapstructure:"sending_queue"`

	DSN string `mapstructure:"dsn"`

	EnableExpHist bool `mapstructure:"enable_exp_hist"`

	Database        string `mapstructure:"database"`
	SamplesTable    string `mapstructure:"samples_table"`
	TimeSeriesTable string `mapstructure:"time_series_table"`
	ExpHistTable    string `mapstructure:"exp_hist_table"`
	MetadataTable   string `mapstructure:"metadata_table"`

	Reduction ReductionConfig `mapstructure:"reduction"`

	SeriesCache SeriesCacheConfig `mapstructure:"series_cache"`

	// MetadataWriteSampleRatio, in (0, 1], is the fraction of metadata rows
	// written each batch. 1.0 (default) writes every row, keeping the attribute
	// catalog complete. Lowering it trades completeness for fewer writes at
	// extreme ingest: a row recurring across batches still lands quickly, but
	// genuinely rare attribute values may be delayed or missed. Opt-in only.
	MetadataWriteSampleRatio float64 `mapstructure:"metadata_write_sample_ratio"`
}

// SeriesCacheConfig bounds the in-memory cache that dedups time-series writes so
// the same (series, hour) row isn't re-sent to ClickHouse within the window.
type SeriesCacheConfig struct {
	// MaxCost caps how many (series, hour) entries are kept; beyond it the cache
	// evicts least-valuable entries. An eviction only causes a redundant row that
	// the ReplacingMergeTree dedups — never a dropped write. Raising it improves
	// dedup (fewer ClickHouse writes) at the cost of memory; lowering it trades
	// the other way. Size it near the active (series, hour) cardinality.
	MaxCost int64 `mapstructure:"max_cost"`
	// NumCounters sizes the admission sketch (eagerly allocated, ~2 bytes each).
	// ristretto recommends ~10x MaxCost.
	NumCounters int64 `mapstructure:"num_counters"`
}

// ReductionConfig configures cardinality control. When enabled, samples and
// series land in the buffer tables with a reduced fingerprint computed from
// per-metric label-drop rules, which are polled from a ClickHouse table.
type ReductionConfig struct {
	Enabled bool `mapstructure:"enabled"`
	// PollInterval is how often the rules table is re-read. Rules carry an
	// effective_from timestamp set ahead by the writer, so the exact poll
	// cadence does not affect correctness as long as it stays well within
	// that margin.
	PollInterval time.Duration `mapstructure:"poll_interval"`
	RulesTable   string        `mapstructure:"rules_table"`
	// BufferSamplesTable and BufferTimeSeriesTable replace SamplesTable and
	// TimeSeriesTable as the write targets when reduction is enabled.
	BufferSamplesTable    string `mapstructure:"buffer_samples_table"`
	BufferTimeSeriesTable string `mapstructure:"buffer_time_series_table"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the exporter configuration is valid
func (cfg *Config) Validate() error {
	if cfg.DSN == "" {
		return errors.New("dsn must be specified")
	}
	if qc := cfg.QueueBatchConfig.Get(); qc != nil {
		if err := qc.Validate(); err != nil {
			return err
		}
	}

	if err := cfg.TimeoutConfig.Validate(); err != nil {
		return err
	}

	if err := cfg.BackOffConfig.Validate(); err != nil {
		return err
	}

	if cfg.SeriesCache.MaxCost <= 0 {
		return errors.New("series_cache.max_cost must be positive")
	}
	if cfg.SeriesCache.NumCounters <= 0 {
		return errors.New("series_cache.num_counters must be positive")
	}

	if cfg.MetadataWriteSampleRatio <= 0 || cfg.MetadataWriteSampleRatio > 1 {
		return errors.New("metadata_write_sample_ratio must be in (0, 1]")
	}

	if cfg.Reduction.Enabled {
		if cfg.Reduction.PollInterval < 5*time.Second {
			return errors.New("reduction.poll_interval must be at least 5s")
		}
		if cfg.Reduction.RulesTable == "" {
			return errors.New("reduction.rules_table must be specified")
		}
		if cfg.Reduction.BufferSamplesTable == "" {
			return errors.New("reduction.buffer_samples_table must be specified")
		}
		if cfg.Reduction.BufferTimeSeriesTable == "" {
			return errors.New("reduction.buffer_time_series_table must be specified")
		}
	}

	return nil
}

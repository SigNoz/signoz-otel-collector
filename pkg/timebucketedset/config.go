package timebucketedset

import (
	"errors"
	"fmt"
	"time"
)

const (
	defaultMaxBuckets    int = 3
	defaultMaxBucketSize int = 256 << 20
	// fastcache spreads its budget over 512 internal buckets of at least one
	// 64 KiB chunk each.
	minMaxBucketSize int = 32 << 20
)

type Config struct {
	// MaxBuckets is the number of buckets to create in the bucketset.
	MaxBuckets int

	// MaxBucketSize is the fastcache chunk budget per bucket. A bucket holds about MaxBytes/(len(key)+4) identities before it starts evicting the oldest.
	MaxBucketSize int

	// PreWriteWindow is the tail of each bucket during which identities already registered for it are also registered for the next bucket.
	PreWriteWindow time.Duration
}

func DefaultConfig() Config {
	return Config{
		MaxBuckets:     defaultMaxBuckets,
		MaxBucketSize:  defaultMaxBucketSize,
		PreWriteWindow: 0, // disables pre write.
	}
}

func (c Config) WithDefaults() Config {
	if c.MaxBuckets == 0 {
		c.MaxBuckets = defaultMaxBuckets
	}

	if c.MaxBucketSize == 0 {
		c.MaxBucketSize = defaultMaxBucketSize
	}

	return c
}

func (c Config) Validate(width time.Duration) error {
	if width <= 0 {
		return errors.New("time_bucketed_set::width must be positive")
	}

	if c.MaxBuckets < 2 {
		return errors.New("time_bucketed_set::max_buckets must be at least 2, one for the current and one for the next")
	}

	if c.MaxBucketSize < minMaxBucketSize {
		return fmt.Errorf("time_bucketed_set::max_bucket_size must be at least %d", minMaxBucketSize)
	}

	if c.PreWriteWindow < 0 || c.PreWriteWindow >= width {
		return errors.New("time_bucketed_set::pre_write_window must be shorter than the bucket width")
	}

	if c.PreWriteWindow > 0 && c.PreWriteWindow < time.Second {
		return errors.New("time_bucketed_set::pre_write_window must be at least 1s")
	}

	return nil
}

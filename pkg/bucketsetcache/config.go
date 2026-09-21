package bucketsetcache

import (
	"errors"
	"fmt"
	"time"
)

const (
	defaultMaxBuckets = 3
	defaultMaxBytes   = 256 << 20
	// fastcache spreads its budget over 512 internal buckets of at least one
	// 64 KiB chunk each.
	minMaxBytes = 32 << 20
)

type Config struct {
	MaxBuckets int `mapstructure:"max_buckets"`
	// MaxBytes is the fastcache chunk budget per bucket. A bucket holds about
	// MaxBytes/(len(key)+4) identities before it starts evicting the oldest.
	MaxBytes int `mapstructure:"max_bytes"`
	// PreWriteWindow is the tail of each bucket during which identities already
	// registered for it are also registered for the next bucket. 0 disables.
	PreWriteWindow time.Duration `mapstructure:"pre_write_window"`
}

func DefaultConfig() Config {
	return Config{
		MaxBuckets: defaultMaxBuckets,
		MaxBytes:   defaultMaxBytes,
	}
}

func (c Config) withDefaults() Config {
	if c.MaxBuckets == 0 {
		c.MaxBuckets = defaultMaxBuckets
	}
	if c.MaxBytes == 0 {
		c.MaxBytes = defaultMaxBytes
	}
	return c
}

// Validate applies defaults first, so an absent config block is valid.
func (c Config) Validate(width time.Duration) error {
	c = c.withDefaults()
	if width <= 0 {
		return errors.New("bucketsetcache: width must be positive")
	}
	if c.MaxBuckets < 2 {
		return errors.New("bucketsetcache: max_buckets must be at least 2")
	}
	if c.MaxBytes < minMaxBytes {
		return fmt.Errorf("bucketsetcache: max_bytes must be at least %d", minMaxBytes)
	}
	if c.PreWriteWindow < 0 || c.PreWriteWindow >= width {
		return errors.New("bucketsetcache: pre_write_window must be shorter than the bucket width")
	}
	if c.PreWriteWindow > 0 && c.PreWriteWindow < time.Second {
		return errors.New("bucketsetcache: pre_write_window must be at least 1s")
	}
	return nil
}

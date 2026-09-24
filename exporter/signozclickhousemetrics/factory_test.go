package signozclickhousemetrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	require.NotNil(t, cfg)
	timeBucketedSet := cfg.(*Config).TimeBucketedSet
	assert.False(t, timeBucketedSet.Enabled)
	assert.Equal(t, 15*time.Minute, timeBucketedSet.PreWriteWindow)
	assert.Equal(t, 3, timeBucketedSet.MaxBuckets)
}

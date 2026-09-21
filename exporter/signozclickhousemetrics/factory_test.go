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
	assert.Equal(t, 10*time.Minute, cfg.(*Config).BucketSetCache.PreWriteWindow)
}

package signozclickhousemetrics

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	cfg := &Config{}
	err := cfg.Validate()
	require.Error(t, err)
}

func TestConfig_Validate_Valid(t *testing.T) {
	cfg := &Config{
		DSN: "tcp://localhost:9000?database=default",
	}
	err := cfg.Validate()
	require.NoError(t, err)
}

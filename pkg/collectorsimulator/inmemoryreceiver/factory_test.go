package inmemoryreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateLogsReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	te, err := factory.CreateLogs(
		context.Background(), receivertest.NewNopSettings(component.MustNewType("memory")), cfg, consumertest.NewNop(),
	)
	assert.NoError(t, err)
	assert.NotNil(t, te)
}

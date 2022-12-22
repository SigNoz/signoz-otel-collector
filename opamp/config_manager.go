package opamp

import (
	"github.com/open-telemetry/opamp-go/protobufs"
	"go.uber.org/zap"
)

// agentConfigManager is responsible for managing the agent configuration
// It is responsible for:
// 1. Reading the agent configuration from the file
// 2. Reloading the agent configuration when the file changes
// 3. Providing the current agent configuration to the Opamp client
type agentConfigManager struct {
	agentConfig *remoteControlledConfig
	logger      *zap.Logger
}

type reloadFunc func([]byte) error

type remoteControlledConfig struct {
	path string

	reloader reloadFunc

	currentHash []byte
}

// createEffectiveConfigMsg creates a protobuf message that contains the effective config.
func (a *agentConfigManager) CreateEffectiveConfigMsg() (*protobufs.EffectiveConfig, error) {
	return nil, nil
}

// Apply applies the remote configuration to the agent.
// By comparing the current hash with the hash in the remote configuration,
// it determines if the configuration has changed.
// If the configuration has changed, it reloads the configuration and returns true.
// If the configuration has not changed, it returns false.
// If there is an error, it returns an error.
// The caller is responsible for sending the updated configuration to the Opamp server.
func (a *agentConfigManager) Apply(remoteConfig *protobufs.AgentRemoteConfig) (bool, error) {
	return false, nil
}

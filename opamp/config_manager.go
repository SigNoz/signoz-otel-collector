package opamp

import (
	"bytes"
	"fmt"
	"os"

	"github.com/open-telemetry/opamp-go/protobufs"
	"go.uber.org/zap"
)

const collectorConfigKey = "collector.yaml"

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

func NewDynamicConfig(configPath string, reloader reloadFunc) (*remoteControlledConfig, error) {
	remoteControlledConfig := &remoteControlledConfig{
		path:     configPath,
		reloader: reloader,
	}

	if err := remoteControlledConfig.UpdateCurrentHash(); err != nil {
		return nil, fmt.Errorf("failed to compute hash for the current config %w", err)
	}

	return remoteControlledConfig, nil
}

func (m *remoteControlledConfig) UpdateCurrentHash() error {
	contents, err := os.ReadFile(m.path)
	if err != nil {
		m.currentHash = fileHash([]byte{})
		return fmt.Errorf("failed to read config file %s: %w", m.path, err)
	}
	m.currentHash = fileHash(contents)
	return nil
}

func NewAgentConfigManager(logger *zap.Logger) *agentConfigManager {
	return &agentConfigManager{
		logger: logger.Named("agent-config-manager"),
	}
}

func (a *agentConfigManager) Set(remoteControlledConfig *remoteControlledConfig) {
	a.agentConfig = remoteControlledConfig
}

// createEffectiveConfigMsg creates a protobuf message that contains the effective config.
func (a *agentConfigManager) CreateEffectiveConfigMsg() (*protobufs.EffectiveConfig, error) {
	configMap := make(map[string]*protobufs.AgentConfigFile, 1)

	body, err := os.ReadFile(a.agentConfig.path)
	if err != nil {
		return nil, fmt.Errorf("error reading config file %s: %w", a.agentConfig.path, err)
	}

	configMap[collectorConfigKey] = &protobufs.AgentConfigFile{
		Body:        body,
		ContentType: "text/yaml",
	}

	return &protobufs.EffectiveConfig{
		ConfigMap: &protobufs.AgentConfigMap{
			ConfigMap: configMap,
		},
	}, nil
}

// Apply applies the remote configuration to the agent.
// By comparing the current hash with the hash in the remote configuration,
// it determines if the configuration has changed.
// If the configuration has changed, it reloads the configuration and returns true.
// If the configuration has not changed, it returns false.
// If there is an error, it returns an error.
// The caller is responsible for sending the updated configuration to the Opamp server.
func (a *agentConfigManager) Apply(remoteConfig *protobufs.AgentRemoteConfig) (bool, error) {
	remoteConfigMap := remoteConfig.GetConfig().GetConfigMap()

	if remoteConfigMap == nil {
		return false, nil
	}

	remoteCollectorConfig, ok := remoteConfigMap[collectorConfigKey]

	if !ok {
		return false, nil
	}

	return a.applyRemoteConfig(a.agentConfig, remoteCollectorConfig.GetBody())
}

// applyRemoteConfig applies the remote config to the agent.
func (a *agentConfigManager) applyRemoteConfig(currentConfig *remoteControlledConfig, newContents []byte) (changed bool, err error) {
	newConfigHash := fileHash(newContents)

	if bytes.Equal(currentConfig.currentHash, newConfigHash) {
		return false, nil
	}

	err = currentConfig.reloader(newContents)
	if err != nil {
		return false, fmt.Errorf("failed to reload config: %s: %w", currentConfig.path, err)
	}

	err = currentConfig.UpdateCurrentHash()
	if err != nil {
		err = fmt.Errorf("failed hash compute for config %s: %w", currentConfig.path, err)
		return true, err
	}

	return true, nil
}

package opamp

import (
	"context"

	"github.com/SigNoz/signoz-otel-collector/signozcol"
	"github.com/oklog/ulid"
	"github.com/open-telemetry/opamp-go/client"
	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
	"go.uber.org/zap"
)

// serverClient is the implementation of the Opamp client
// that connects to the Opamp server and manages the agent configuration
// and the collector lifecycle.
// It implements the client.OpAMPClient interface.
// It is responsible for:
// 1. Connecting to the Opamp server
// 2. Sending the current agent configuration to the Opamp server
// 3. Receiving the remote configuration from the Opamp server
// 4. Applying the remote configuration to the agent
// 5. Sending the updated agent configuration to the Opamp server
type serverClient struct {
	logger        *zap.Logger
	opampClient   client.OpAMPClient
	configManager *agentConfigManager
	collector     signozcol.WrappedCollector
	managerConfig AgentManagerConfig
	instanceId    ulid.ULID
}

// Start starts the Opamp client
// It connects to the Opamp server and starts the Opamp client
func (s *serverClient) Start(ctx context.Context) error {
	return nil
}

// Stop stops the Opamp client
// It stops the Opamp client and disconnects from the Opamp server
func (s *serverClient) Stop(ctx context.Context) error {
	return nil
}

// onMessageFuncHandler is the callback function that is called when the Opamp client receives a message from the Opamp server
func (s *serverClient) onMessageFuncHandler(ctx context.Context, msg *types.MessageData) {
}

// onRemoteConfigHandler is the callback function that is called when the Opamp client receives a remote configuration from the Opamp server
func (s *serverClient) onRemoteConfigHandler(ctx context.Context, remoteConfig *protobufs.AgentRemoteConfig) error {
	return nil
}

// reload is the callback function that is called when the agent configuration file changes
func (s *serverClient) reload(contents []byte) error {
	return nil
}

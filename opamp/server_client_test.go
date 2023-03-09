package opamp

import (
	"context"
	"math/rand"
	"net/url"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SigNoz/signoz-otel-collector/signozcol"
	"github.com/gorilla/websocket"
	"github.com/oklog/ulid"
	"github.com/open-telemetry/opamp-go/client"
	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func eventually(t *testing.T, f func() bool) {
	assert.Eventually(t, f, 5*time.Second, 10*time.Millisecond)
}

func prepareSettings(t *testing.T, settings *types.StartSettings, c client.OpAMPClient) {
	// Autogenerate instance id.
	entropy := ulid.Monotonic(rand.New(rand.NewSource(99)), 0)
	settings.InstanceUid = ulid.MustNew(ulid.Timestamp(time.Now()), entropy).String()

	// Make sure correct URL scheme is used, based on the type of the OpAMP client.
	u, err := url.Parse(settings.OpAMPServerURL)
	require.NoError(t, err)
	u.Scheme = "ws"
	settings.OpAMPServerURL = u.String()
}

func createAgentDescr() *protobufs.AgentDescription {
	agentDescr := &protobufs.AgentDescription{
		IdentifyingAttributes: []*protobufs.KeyValue{
			{
				Key:   "host.name",
				Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{StringValue: "somehost"}},
			},
		},
	}
	return agentDescr
}

func prepareClient(t *testing.T, settings *types.StartSettings, c client.OpAMPClient) {
	prepareSettings(t, settings, c)
	err := c.SetAgentDescription(createAgentDescr())
	assert.NoError(t, err)
}

func startClient(t *testing.T, settings types.StartSettings, client client.OpAMPClient) {
	prepareClient(t, &settings, client)
	prepareClient(t, &settings, client)
	err := client.Start(context.Background(), settings)
	assert.NoError(t, err)
}

func TestNewClient(t *testing.T) {
	// FIXME(srikanthccv): this test interferes with other tests that use the same port.
	// Remove the sleep and find a better way to fix this.
	time.Sleep(5 * time.Second)
	srv := StartMockServer(t)

	var conn atomic.Value
	srv.OnWSConnect = func(c *websocket.Conn) {
		conn.Store(c)
	}
	var connected int64

	srv.OnMessage = func(msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
		fileContents, err := os.ReadFile("testdata/coll-config-path-changed.yaml")
		assert.NoError(t, err)
		atomic.AddInt64(&connected, 1)
		var resp *protobufs.ServerToAgent = &protobufs.ServerToAgent{}
		if atomic.LoadInt64(&connected) == 1 {
			resp = &protobufs.ServerToAgent{
				RemoteConfig: &protobufs.AgentRemoteConfig{
					Config: &protobufs.AgentConfigMap{
						ConfigMap: map[string]*protobufs.AgentConfigFile{
							"collector.yaml": {
								Body:        fileContents,
								ContentType: "text/yaml",
							},
						},
					},
				},
			}
		}
		return resp
	}

	logger := zap.NewNop()
	cnt := 0
	reloadFunc := func(contents []byte) error {
		cnt++
		return nil
	}

	_, err := NewDynamicConfig("./testdata/coll-config-path.yaml", reloadFunc)
	require.NoError(t, err)

	// maintain a cop of the original config file and restore it after the test
	fileContents, err := os.ReadFile("testdata/coll-config-path.yaml")
	require.NoError(t, err)
	defer func() {
		err := os.WriteFile("testdata/coll-config-path.yaml", fileContents, 0644)
		require.NoError(t, err)
	}()

	coll := signozcol.New(signozcol.WrappedCollectorSettings{
		ConfigPaths: []string{"./testdata/coll-config-path.yaml"},
		Version:     "0.0.1-server-client-test",
	})

	svrClient, err := NewServerClient(&NewServerClientOpts{
		Logger: logger,
		Config: &AgentManagerConfig{
			ServerEndpoint: "ws://" + srv.Endpoint,
		},
		CollectorConfgPath: "./testdata/coll-config-path.yaml",
		WrappedCollector:   coll,
	})

	require.NoError(t, err)

	ctx := context.Background()
	// Start the client.
	err = svrClient.Start(ctx)
	defer func() {
		err := svrClient.Stop(ctx)
		require.NoError(t, err)
	}()
	require.NoError(t, err)

	// Wait for the client to connect.
	eventually(t, func() bool {
		return atomic.LoadInt64(&connected) == 1
	})
}

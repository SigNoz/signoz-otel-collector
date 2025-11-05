// Brought in as is from opentelemetry-collector-contrib

package json

import (
	"context"
	"testing"
	"time"

	signozstanzaentry "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/entry"
	signozstanzahelper "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator/helper"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func newTestParser(t *testing.T) *Parser {
	config := NewConfigWithID("test")
	set := componenttest.NewNopTelemetrySettings()
	op, err := config.Build(set)
	require.NoError(t, err)
	return op.(*Parser)
}

func TestConfigBuild(t *testing.T) {
	config := NewConfigWithID("test")
	set := componenttest.NewNopTelemetrySettings()
	op, err := config.Build(set)
	require.NoError(t, err)
	require.IsType(t, &Parser{}, op)
}

func TestConfigBuildFailure(t *testing.T) {
	config := NewConfigWithID("test")
	config.OnError = "invalid_on_error"
	set := componenttest.NewNopTelemetrySettings()
	_, err := config.Build(set)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid `on_error` field")
}

func TestParserStringFailure(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parse("invalid")
	require.Error(t, err)
	require.Contains(t, err.Error(), "expected { character for map value")
}

func TestParserByteFailure(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parse([]byte("invalid"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "type []uint8 cannot be parsed as JSON")
}

func TestParserInvalidType(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parse([]int{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "type []int cannot be parsed as JSON")
}

func TestJSONImplementations(t *testing.T) {
	require.Implements(t, (*operator.Operator)(nil), new(Parser))
}

var testJSONPayload = `{
  "stream": "stdout",
  "_p": "F",
  "log": "{\"level\":\"INFO\",\"target\":\"amzn_nfm::events::event_provider_ebpf\"}",
  "log_processed": {
    "level": "INFO",
    "message": "Under log_processed",
    "target": "amzn_nfm::events::event_provider_ebpf",
    "timestamp": 1748426199363
  },
  "kubernetes": {
    "pod_name": "aws-network-flow-monitor-agent-qdrt2",
    "namespace_name": "amazon-network-flow-monitor",
    "pod_id": "c514f9a4-0412-4dd7-a4cb-7ff51d9ddee9",
    "host": "ip-172-31-29-49.ap-south-1.compute.internal",
    "container_name": "aws-network-flow-monitor-agent",
    "docker_id": "257e614a0a24c811d9d56b2ae6245b8ae29a1cd3023f3f8a550164108f1fd128",
    "container_hash": "602401143452.dkr.ecr.ap-south-1.amazonaws.com/aws-network-sonar-agent@sha256:13bc6a5d47f0fc196e969159676dcb52a1eadbe5097b952a1b53bc449c525ed2",
    "container_image": "602401143452.dkr.ecr.ap-south-1.amazonaws.com/aws-network-sonar-agent:v1.0.2-eksbuild.4"
  },
  "docker": [
    "container_1",
    "container_8"
  ],
  "valorant": {
    "game": {
      "is_game": "false",
      "metadata": {
        "version": "v0.0.1",
        "installation_path": "C://games/installed/valorant",
        "vanguard": {
          "running": true,
          "malformed_hardware": false,
          "version": "patch_v1.100.0",
          "hash_check_status": "success"
        }
      }
    },
    "uninstall": true,
    "message": "under valorant 3"
  }
}`

func TestParser(t *testing.T) {
	cases := []struct {
		name      string
		configure func(*Config)
		input     *entry.Entry
		expect    *entry.Entry
	}{
		{
			"simple",
			func(_ *Config) {},
			&entry.Entry{
				Body: `{}`,
			},
			&entry.Entry{
				Attributes: map[string]any{},
				Body:       `{}`,
			},
		},
		{
			"nested",
			func(_ *Config) {},
			&entry.Entry{
				Body: `{"superkey":"superval"}`,
			},
			&entry.Entry{
				Attributes: map[string]any{
					"superkey": "superval",
				},
				Body: `{"superkey":"superval"}`,
			},
		},
		{
			"received_map",
			func(_ *Config) {},
			&entry.Entry{
				Body: map[string]any{
					"superkey": "superval",
					"superkey2": map[string]any{
						"nestedkey1": "nestedval",
					},
					"superkey3": []string{"val1", "val2"},
				},
			},
			&entry.Entry{
				Attributes: map[string]any{
					"superkey": "superval",
					"superkey2": map[string]any{
						"nestedkey1": "nestedval",
					},
					"superkey3": []string{"val1", "val2"},
				},
				// Note: body is not stringified by parser but the exporter when saving in clickhouse
				Body: map[string]any{
					"superkey": "superval",
					"superkey2": map[string]any{
						"nestedkey1": "nestedval",
					},
					"superkey3": []string{"val1", "val2"},
				},
			},
		},
		{
			"with_timestamp",
			func(p *Config) {
				parseFrom := signozstanzaentry.Field{FieldInterface: entry.NewAttributeField("timestamp")}
				p.TimeParser = &signozstanzahelper.TimeParser{
					ParseFrom:  &parseFrom,
					LayoutType: "epoch",
					Layout:     "s",
				}
			},
			&entry.Entry{
				Body: `{"superkey":"superval","timestamp":1136214245}`,
			},
			&entry.Entry{
				Attributes: map[string]any{
					"superkey":  "superval",
					"timestamp": float64(1136214245),
				},
				Body:      `{"superkey":"superval","timestamp":1136214245}`,
				Timestamp: time.Unix(1136214245, 0),
			},
		},
		{
			"with_scope",
			func(p *Config) {
				p.ScopeNameParser = &signozstanzahelper.ScopeNameParser{
					ParseFrom: signozstanzaentry.Field{FieldInterface: entry.NewAttributeField("logger_name")},
				}
			},
			&entry.Entry{
				Body: `{"superkey":"superval","logger_name":"logger"}`,
			},
			&entry.Entry{
				Attributes: map[string]any{
					"superkey":    "superval",
					"logger_name": "logger",
				},
				Body:      `{"superkey":"superval","logger_name":"logger"}`,
				ScopeName: "logger",
			},
		},
		{
			"simple_json_test",
			func(c *Config) {
				c.EnableFlattening = false
			},
			&entry.Entry{
				Body: testJSONPayload,
			},
			&entry.Entry{
				Attributes: map[string]any{
					"_p":     "F",
					"docker": []any{"container_1", "container_8"},
					"kubernetes": map[string]any{
						"container_hash":  "602401143452.dkr.ecr.ap-south-1.amazonaws.com/aws-network-sonar-agent@sha256:13bc6a5d47f0fc196e969159676dcb52a1eadbe5097b952a1b53bc449c525ed2",
						"container_image": "602401143452.dkr.ecr.ap-south-1.amazonaws.com/aws-network-sonar-agent:v1.0.2-eksbuild.4", "container_name": "aws-network-flow-monitor-agent",
						"docker_id":      "257e614a0a24c811d9d56b2ae6245b8ae29a1cd3023f3f8a550164108f1fd128",
						"host":           "ip-172-31-29-49.ap-south-1.compute.internal",
						"namespace_name": "amazon-network-flow-monitor",
						"pod_id":         "c514f9a4-0412-4dd7-a4cb-7ff51d9ddee9",
						"pod_name":       "aws-network-flow-monitor-agent-qdrt2",
					},
					"log": "{\"level\":\"INFO\",\"target\":\"amzn_nfm::events::event_provider_ebpf\"}",
					"log_processed": map[string]any{
						"level":     "INFO",
						"message":   "Under log_processed",
						"target":    "amzn_nfm::events::event_provider_ebpf",
						"timestamp": 1.748426199363e+12,
					},
					"stream": "stdout",
					"valorant": map[string]any{
						"game": map[string]any{
							"is_game": "false",
							"metadata": map[string]any{
								"installation_path": "C://games/installed/valorant",
								"vanguard": map[string]any{
									"hash_check_status":  "success",
									"malformed_hardware": false,
									"running":            true,
									"version":            "patch_v1.100.0",
								},
								"version": "v0.0.1",
							},
						},
						"uninstall": true,
						"message":   "under valorant 3",
					},
				},
				Body: testJSONPayload,
			},
		},
		{
			"enable_flattening_and_path",
			func(c *Config) {
				c.EnableFlattening = true
				c.EnablePaths = true
				c.MaxFlatteningDepth = 1
			},
			&entry.Entry{
				Body: testJSONPayload,
			},
			&entry.Entry{
				Attributes: map[string]any{
					"stream":                     "stdout",
					"_p":                         "F",
					"log":                        "{\"level\":\"INFO\",\"target\":\"amzn_nfm::events::event_provider_ebpf\"}",
					"log_processed.level":        "INFO",
					"log_processed.message":      "Under log_processed",
					"log_processed.target":       "amzn_nfm::events::event_provider_ebpf",
					"log_processed.timestamp":    1.748426199363e+12,
					"kubernetes.pod_name":        "aws-network-flow-monitor-agent-qdrt2",
					"kubernetes.namespace_name":  "amazon-network-flow-monitor",
					"kubernetes.pod_id":          "c514f9a4-0412-4dd7-a4cb-7ff51d9ddee9",
					"kubernetes.host":            "ip-172-31-29-49.ap-south-1.compute.internal",
					"kubernetes.container_name":  "aws-network-flow-monitor-agent",
					"kubernetes.docker_id":       "257e614a0a24c811d9d56b2ae6245b8ae29a1cd3023f3f8a550164108f1fd128",
					"kubernetes.container_hash":  "602401143452.dkr.ecr.ap-south-1.amazonaws.com/aws-network-sonar-agent@sha256:13bc6a5d47f0fc196e969159676dcb52a1eadbe5097b952a1b53bc449c525ed2",
					"kubernetes.container_image": "602401143452.dkr.ecr.ap-south-1.amazonaws.com/aws-network-sonar-agent:v1.0.2-eksbuild.4",

					"docker": []any{
						"container_1",
						"container_8",
					},
					"valorant.game": map[string]any{
						"is_game": "false",
						"metadata": map[string]any{
							"version":           "v0.0.1",
							"installation_path": "C://games/installed/valorant",
							"vanguard": map[string]any{
								"running":            true,
								"malformed_hardware": false,
								"version":            "patch_v1.100.0",
								"hash_check_status":  "success",
							},
						},
					},
					"valorant.uninstall": true,
					"valorant.message":   "under valorant 3",
				},
				Body: testJSONPayload,
			},
		},
		{
			"enable_flattening_and_path_level_2",
			func(c *Config) {
				c.EnableFlattening = true
				c.EnablePaths = true
				c.MaxFlatteningDepth = 2
			},
			&entry.Entry{
				Body: testJSONPayload,
			},
			&entry.Entry{
				Attributes: map[string]any{
					"_p":                         "F",
					"docker":                     []any{"container_1", "container_8"},
					"kubernetes.container_hash":  "602401143452.dkr.ecr.ap-south-1.amazonaws.com/aws-network-sonar-agent@sha256:13bc6a5d47f0fc196e969159676dcb52a1eadbe5097b952a1b53bc449c525ed2",
					"kubernetes.container_image": "602401143452.dkr.ecr.ap-south-1.amazonaws.com/aws-network-sonar-agent:v1.0.2-eksbuild.4",
					"kubernetes.container_name":  "aws-network-flow-monitor-agent",
					"kubernetes.docker_id":       "257e614a0a24c811d9d56b2ae6245b8ae29a1cd3023f3f8a550164108f1fd128",
					"kubernetes.host":            "ip-172-31-29-49.ap-south-1.compute.internal",
					"kubernetes.namespace_name":  "amazon-network-flow-monitor",
					"kubernetes.pod_id":          "c514f9a4-0412-4dd7-a4cb-7ff51d9ddee9",
					"kubernetes.pod_name":        "aws-network-flow-monitor-agent-qdrt2",
					"log":                        "{\"level\":\"INFO\",\"target\":\"amzn_nfm::events::event_provider_ebpf\"}",
					"log_processed.level":        "INFO",
					"log_processed.message":      "Under log_processed",
					"log_processed.target":       "amzn_nfm::events::event_provider_ebpf",
					"log_processed.timestamp":    1.748426199363e+12,
					"stream":                     "stdout",
					"valorant.game.is_game":      "false",
					"valorant.game.metadata": map[string]any{
						"installation_path": "C://games/installed/valorant",
						"vanguard": map[string]any{
							"hash_check_status":  "success",
							"malformed_hardware": false,
							"running":            true,
							"version":            "patch_v1.100.0",
						},
						"version": "v0.0.1",
					},
					"valorant.message":   "under valorant 3",
					"valorant.uninstall": true,
				},
				Body: testJSONPayload,
			},
		},
		{
			"enable_flattening_and_path_level_4",
			func(c *Config) {
				c.EnableFlattening = true
				c.EnablePaths = true
				c.MaxFlatteningDepth = 4
				c.PathPrefix = "flattened"
			},
			&entry.Entry{
				Body: testJSONPayload,
			},
			&entry.Entry{
				Attributes: map[string]any{
					"flattened._p":                                                 "F",
					"flattened.docker":                                             []any{"container_1", "container_8"},
					"flattened.kubernetes.container_hash":                          "602401143452.dkr.ecr.ap-south-1.amazonaws.com/aws-network-sonar-agent@sha256:13bc6a5d47f0fc196e969159676dcb52a1eadbe5097b952a1b53bc449c525ed2",
					"flattened.kubernetes.container_image":                         "602401143452.dkr.ecr.ap-south-1.amazonaws.com/aws-network-sonar-agent:v1.0.2-eksbuild.4",
					"flattened.kubernetes.container_name":                          "aws-network-flow-monitor-agent",
					"flattened.kubernetes.docker_id":                               "257e614a0a24c811d9d56b2ae6245b8ae29a1cd3023f3f8a550164108f1fd128",
					"flattened.kubernetes.host":                                    "ip-172-31-29-49.ap-south-1.compute.internal",
					"flattened.kubernetes.namespace_name":                          "amazon-network-flow-monitor",
					"flattened.kubernetes.pod_id":                                  "c514f9a4-0412-4dd7-a4cb-7ff51d9ddee9",
					"flattened.kubernetes.pod_name":                                "aws-network-flow-monitor-agent-qdrt2",
					"flattened.log":                                                "{\"level\":\"INFO\",\"target\":\"amzn_nfm::events::event_provider_ebpf\"}",
					"flattened.log_processed.level":                                "INFO",
					"flattened.log_processed.message":                              "Under log_processed",
					"flattened.log_processed.target":                               "amzn_nfm::events::event_provider_ebpf",
					"flattened.log_processed.timestamp":                            1.748426199363e+12,
					"flattened.stream":                                             "stdout",
					"flattened.valorant.game.is_game":                              "false",
					"flattened.valorant.game.metadata.installation_path":           "C://games/installed/valorant",
					"flattened.valorant.game.metadata.vanguard.hash_check_status":  "success",
					"flattened.valorant.game.metadata.vanguard.malformed_hardware": false,
					"flattened.valorant.game.metadata.vanguard.running":            true,
					"flattened.valorant.game.metadata.vanguard.version":            "patch_v1.100.0",
					"flattened.valorant.game.metadata.version":                     "v0.0.1",
					"flattened.valorant.message":                                   "under valorant 3",
					"flattened.valorant.uninstall":                                 true,
				},
				Body: testJSONPayload,
			},
		},
		{
			"enable_flattening_and_disable_paths",
			func(c *Config) {
				c.EnableFlattening = true
				c.EnablePaths = false
				c.MaxFlatteningDepth = 4
			},
			&entry.Entry{
				Body: testJSONPayload,
			},
			&entry.Entry{
				Attributes: map[string]any{
					"_p":                 "F",
					"container_hash":     "602401143452.dkr.ecr.ap-south-1.amazonaws.com/aws-network-sonar-agent@sha256:13bc6a5d47f0fc196e969159676dcb52a1eadbe5097b952a1b53bc449c525ed2",
					"container_image":    "602401143452.dkr.ecr.ap-south-1.amazonaws.com/aws-network-sonar-agent:v1.0.2-eksbuild.4",
					"container_name":     "aws-network-flow-monitor-agent",
					"docker":             []any{"container_1", "container_8"},
					"docker_id":          "257e614a0a24c811d9d56b2ae6245b8ae29a1cd3023f3f8a550164108f1fd128",
					"hash_check_status":  "success",
					"host":               "ip-172-31-29-49.ap-south-1.compute.internal",
					"installation_path":  "C://games/installed/valorant",
					"is_game":            "false",
					"level":              "INFO",
					"log":                "{\"level\":\"INFO\",\"target\":\"amzn_nfm::events::event_provider_ebpf\"}",
					"malformed_hardware": false,
					"message":            "under valorant 3",
					"namespace_name":     "amazon-network-flow-monitor",
					"pod_id":             "c514f9a4-0412-4dd7-a4cb-7ff51d9ddee9",
					"pod_name":           "aws-network-flow-monitor-agent-qdrt2",
					"running":            true,
					"stream":             "stdout",
					"target":             "amzn_nfm::events::event_provider_ebpf",
					"timestamp":          1.748426199363e+12,
					"uninstall":          true,
					"version":            "v0.0.1",
				},
				Body: testJSONPayload,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := NewConfigWithID("test")
			cfg.OutputIDs = []string{"fake"}
			tc.configure(cfg)

			set := componenttest.NewNopTelemetrySettings()
			op, err := cfg.Build(set)
			require.NoError(t, err)

			fake := testutil.NewFakeOutput(t)
			require.NoError(t, op.SetOutputs([]operator.Operator{fake}))

			ots := time.Now()
			tc.input.ObservedTimestamp = ots
			tc.expect.ObservedTimestamp = ots

			err = op.Process(context.Background(), tc.input)
			require.NoError(t, err)
			fake.ExpectEntry(t, tc.expect)
		})
	}
}

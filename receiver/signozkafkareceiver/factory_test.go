// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signozkafkareceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/SigNoz/signoz-otel-collector/receiver/signozkafkareceiver/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
	assert.Equal(t, []string{defaultBroker}, cfg.Brokers)
	assert.Equal(t, defaultTopic, cfg.Topic)
	assert.Equal(t, defaultGroupID, cfg.GroupID)
	assert.Equal(t, defaultClientID, cfg.ClientID)
	assert.Equal(t, defaultInitialOffset, cfg.InitialOffset)
}

func TestCreateTracesReceiver(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Brokers = []string{"invalid:9092"}
	cfg.ProtocolVersion = "2.0.0"
	// Disable metadata collection to prevent real client creation
	cfg.Metadata.Full = false
	// Set very short timeouts to fail fast
	cfg.Metadata.Retry.Max = 1
	cfg.Metadata.Retry.Backoff = 1 * time.Millisecond

	f := kafkaReceiverFactory{tracesUnmarshalers: defaultTracesUnmarshalers()}

	// Since we're disabling metadata.Full, it should succeed but create a receiver
	// that will fail when started
	r, err := f.createTracesReceiver(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
	require.NoError(t, err)
	require.NotNil(t, r)

	// Clean up the receiver
	if receiver, ok := r.(*kafkaTracesConsumer); ok && receiver.consumerGroup != nil {
		_ = receiver.consumerGroup.Close()
	}
}

func TestCreateTracesReceiver_error(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.ProtocolVersion = "2.0.0"
	// disable contacting broker at startup
	cfg.Metadata.Full = false
	f := kafkaReceiverFactory{tracesUnmarshalers: defaultTracesUnmarshalers()}
	r, err := f.createTracesReceiver(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
	require.NoError(t, err)
	assert.NotNil(t, r)

	// Clean up the receiver
	if receiver, ok := r.(*kafkaTracesConsumer); ok && receiver.consumerGroup != nil {
		_ = receiver.consumerGroup.Close()
	}
}

func TestWithTracesUnmarshalers(t *testing.T) {
	unmarshaler := &customTracesUnmarshaler{}
	f := NewFactory(WithTracesUnmarshalers(unmarshaler))
	cfg := createDefaultConfig().(*Config)
	// disable contacting broker
	cfg.Metadata.Full = false
	cfg.ProtocolVersion = "2.0.0"

	t.Run("custom_encoding", func(t *testing.T) {
		cfg.Encoding = unmarshaler.Encoding()
		receiver, err := f.CreateTraces(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, receiver)

		// Clean up
		if r, ok := receiver.(*kafkaTracesConsumer); ok && r.consumerGroup != nil {
			_ = r.consumerGroup.Close()
		}
	})
	t.Run("default_encoding", func(t *testing.T) {
		cfg.Encoding = defaultEncoding
		receiver, err := f.CreateTraces(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		require.NoError(t, err)
		assert.NotNil(t, receiver)

		// Clean up
		if r, ok := receiver.(*kafkaTracesConsumer); ok && r.consumerGroup != nil {
			_ = r.consumerGroup.Close()
		}
	})
}

func TestCreateMetricsReceiver(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Brokers = []string{"invalid:9092"}
	cfg.ProtocolVersion = "2.0.0"
	// Disable metadata collection to prevent real client creation
	cfg.Metadata.Full = false
	// Set very short timeouts to fail fast
	cfg.Metadata.Retry.Max = 1
	cfg.Metadata.Retry.Backoff = 1 * time.Millisecond

	f := kafkaReceiverFactory{metricsUnmarshalers: defaultMetricsUnmarshalers()}

	// Since we're disabling metadata.Full, it should succeed
	r, err := f.createMetricsReceiver(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
	require.NoError(t, err)
	require.NotNil(t, r)

	// Clean up the receiver
	if receiver, ok := r.(*kafkaMetricsConsumer); ok && receiver.consumerGroup != nil {
		_ = receiver.consumerGroup.Close()
	}
}

func TestCreateMetricsReceiver_error(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.ProtocolVersion = "2.0.0"
	// disable contacting broker at startup
	cfg.Metadata.Full = false
	f := kafkaReceiverFactory{metricsUnmarshalers: defaultMetricsUnmarshalers()}
	r, err := f.createMetricsReceiver(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
	require.NoError(t, err)
	assert.NotNil(t, r)

	// Clean up the receiver
	if receiver, ok := r.(*kafkaMetricsConsumer); ok && receiver.consumerGroup != nil {
		_ = receiver.consumerGroup.Close()
	}
}

func TestWithMetricsUnmarshalers(t *testing.T) {
	unmarshaler := &customMetricsUnmarshaler{}
	f := NewFactory(WithMetricsUnmarshalers(unmarshaler))
	cfg := createDefaultConfig().(*Config)
	// disable contacting broker
	cfg.Metadata.Full = false
	cfg.ProtocolVersion = "2.0.0"

	t.Run("custom_encoding", func(t *testing.T) {
		cfg.Encoding = unmarshaler.Encoding()
		receiver, err := f.CreateMetrics(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, receiver)

		// Clean up
		if r, ok := receiver.(*kafkaMetricsConsumer); ok && r.consumerGroup != nil {
			_ = r.consumerGroup.Close()
		}
	})
	t.Run("default_encoding", func(t *testing.T) {
		cfg.Encoding = defaultEncoding
		receiver, err := f.CreateMetrics(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		require.NoError(t, err)
		assert.NotNil(t, receiver)

		// Clean up
		if r, ok := receiver.(*kafkaMetricsConsumer); ok && r.consumerGroup != nil {
			_ = r.consumerGroup.Close()
		}
	})
}

func TestCreateLogsReceiver(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Brokers = []string{"invalid:9092"}
	cfg.ProtocolVersion = "2.0.0"
	// Disable metadata collection to prevent real client creation
	cfg.Metadata.Full = false
	// Set very short timeouts to fail fast
	cfg.Metadata.Retry.Max = 1
	cfg.Metadata.Retry.Backoff = 1 * time.Millisecond

	f := kafkaReceiverFactory{logsUnmarshalers: defaultLogsUnmarshalers()}

	// Since we're disabling metadata.Full, it should succeed
	r, err := f.createLogsReceiver(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
	require.NoError(t, err)
	require.NotNil(t, r)

	// Clean up the receiver
	if receiver, ok := r.(*kafkaLogsConsumer); ok && receiver.consumerGroup != nil {
		_ = receiver.consumerGroup.Close()
	}
}

func TestCreateLogsReceiver_error(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.ProtocolVersion = "2.0.0"
	// disable contacting broker at startup
	cfg.Metadata.Full = false
	f := kafkaReceiverFactory{logsUnmarshalers: defaultLogsUnmarshalers()}
	r, err := f.createLogsReceiver(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
	require.NoError(t, err)
	assert.NotNil(t, r)

	// Clean up the receiver
	if receiver, ok := r.(*kafkaLogsConsumer); ok && receiver.consumerGroup != nil {
		_ = receiver.consumerGroup.Close()
	}
}

func TestWithLogsUnmarshalers(t *testing.T) {
	unmarshaler := &customLogsUnmarshaler{}
	f := NewFactory(WithLogsUnmarshalers(unmarshaler))
	cfg := createDefaultConfig().(*Config)
	// disable contacting broker
	cfg.Metadata.Full = false
	cfg.ProtocolVersion = "2.0.0"

	t.Run("custom_encoding", func(t *testing.T) {
		cfg.Encoding = unmarshaler.Encoding()
		receiver, err := f.CreateLogs(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, receiver)

		// Clean up
		if r, ok := receiver.(*kafkaLogsConsumer); ok && r.consumerGroup != nil {
			_ = r.consumerGroup.Close()
		}
	})
	t.Run("default_encoding", func(t *testing.T) {
		cfg.Encoding = defaultEncoding
		receiver, err := f.CreateLogs(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		require.NoError(t, err)
		assert.NotNil(t, receiver)

		// Clean up
		if r, ok := receiver.(*kafkaLogsConsumer); ok && r.consumerGroup != nil {
			_ = r.consumerGroup.Close()
		}
	})
}

type customTracesUnmarshaler struct {
}

type customMetricsUnmarshaler struct {
}

type customLogsUnmarshaler struct {
}

var _ TracesUnmarshaler = (*customTracesUnmarshaler)(nil)

func (c customTracesUnmarshaler) Unmarshal([]byte) (ptrace.Traces, error) {
	panic("implement me")
}

func (c customTracesUnmarshaler) Encoding() string {
	return "custom"
}

func (c customMetricsUnmarshaler) Unmarshal([]byte) (pmetric.Metrics, error) {
	panic("implement me")
}

func (c customMetricsUnmarshaler) Encoding() string {
	return "custom"
}

func (c customLogsUnmarshaler) Unmarshal([]byte) (plog.Logs, error) {
	panic("implement me")
}

func (c customLogsUnmarshaler) Encoding() string {
	return "custom"
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package adapter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/SigNoz/signoz-otel-collector/pkg/stanza/operator"
	"github.com/SigNoz/signoz-otel-collector/pkg/stanza/operator/parser/json"
	"github.com/SigNoz/signoz-otel-collector/pkg/stanza/operator/parser/regex"
)

func TestCreateReceiver(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		factory := NewFactory(TestReceiverType{}, component.StabilityLevelDevelopment)
		cfg := factory.CreateDefaultConfig().(*TestConfig)
		cfg.Operators = []operator.Config{
			{
				Builder: json.NewConfig(),
			},
		}
		receiver, err := factory.CreateLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, consumertest.NewNop())
		require.NoError(t, err, "receiver creation failed")
		require.NotNil(t, receiver, "receiver creation failed")
	})

	t.Run("DecodeOperatorConfigsFailureMissingFields", func(t *testing.T) {
		factory := NewFactory(TestReceiverType{}, component.StabilityLevelDevelopment)
		badCfg := factory.CreateDefaultConfig().(*TestConfig)
		badCfg.Operators = []operator.Config{
			{
				Builder: regex.NewConfig(),
			},
		}
		receiver, err := factory.CreateLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), badCfg, consumertest.NewNop())
		require.Error(t, err, "receiver creation should fail if parser configs aren't valid")
		require.Nil(t, receiver, "receiver creation should fail if parser configs aren't valid")
	})
}

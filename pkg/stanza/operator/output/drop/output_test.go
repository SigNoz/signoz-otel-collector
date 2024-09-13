// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package drop

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/SigNoz/signoz-otel-collector/pkg/stanza/entry"
)

func TestBuildValid(t *testing.T) {
	cfg := NewConfig("test")
	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	require.NoError(t, err)
	require.IsType(t, &Output{}, op)
}

func TestBuildIvalid(t *testing.T) {
	cfg := NewConfig("test")
	set := componenttest.NewNopTelemetrySettings()
	set.Logger = nil
	_, err := cfg.Build(set)
	require.Error(t, err)
	require.Contains(t, err.Error(), "build context is missing a logger")
}

func TestProcess(t *testing.T) {
	cfg := NewConfig("test")
	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	require.NoError(t, err)

	entry := entry.New()
	result := op.Process(context.Background(), entry)
	require.Nil(t, result)
}

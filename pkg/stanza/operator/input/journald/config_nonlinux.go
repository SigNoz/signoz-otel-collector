// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package journald // import "github.com/SigNoz/signoz-otel-collector/pkg/stanza/operator/input/journald"

import (
	"errors"

	"go.opentelemetry.io/collector/component"

	"github.com/SigNoz/signoz-otel-collector/pkg/stanza/operator"
)

func (c Config) Build(_ component.TelemetrySettings) (operator.Operator, error) {
	return nil, errors.New("journald input operator is only supported on linux")
}

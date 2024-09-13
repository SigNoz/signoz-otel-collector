// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package fileconsumer // import "github.com/SigNoz/signoz-otel-collector/pkg/stanza/fileconsumer"

import (
	"context"
)

// Noop on windows because we close files immediately after reading.
func (m *Manager) readLostFiles(ctx context.Context) {
}

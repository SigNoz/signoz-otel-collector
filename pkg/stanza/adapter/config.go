// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package adapter // import "github.com/SigNoz/signoz-otel-collector/pkg/stanza/adapter"

import (
	"time"

	"go.opentelemetry.io/collector/component"

	"github.com/SigNoz/signoz-otel-collector/pkg/stanza/otel-collector-contrib-internal/consumerretry"
	"github.com/SigNoz/signoz-otel-collector/pkg/stanza/operator"
)

// BaseConfig is the common configuration of a stanza-based receiver
type BaseConfig struct {
	Operators      []operator.Config    `mapstructure:"operators"`
	StorageID      *component.ID        `mapstructure:"storage"`
	RetryOnFailure consumerretry.Config `mapstructure:"retry_on_failure"`

	// currently not configurable by users, but available for benchmarking
	numWorkers    int
	maxBatchSize  uint
	flushInterval time.Duration
}

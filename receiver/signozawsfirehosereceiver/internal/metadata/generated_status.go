package metadata

import (
	"go.opentelemetry.io/collector/component"
)

var (
	Type      = component.MustNewType("signozawsfirehose")
	ScopeName = "github.com/SigNoz/signoz-otel-collector/receiver/signozawsfirehosereceiver"
)

const (
	MetricsStability = component.StabilityLevelAlpha
	LogsStability    = component.StabilityLevelAlpha
)

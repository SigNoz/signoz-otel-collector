// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package signozspanmetricsprocessor

import (
	"context"

	"github.com/google/uuid"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/featuregate"
)

const (
	// The value of "type" key in configuration.
	typeStr = "signozspanmetrics"
	// The stability level of the processor.
	stability = component.StabilityLevelBeta

	signozID = "signoz.collector.id"
)

// NewFactory creates a factory for the spanmetrics processor.
func NewFactory() component.ProcessorFactory {
	return component.NewProcessorFactory(
		typeStr,
		createDefaultConfig,
		component.WithTracesProcessor(createTracesProcessor, stability),
	)
}

func createDefaultConfig() component.ProcessorConfig {
	return &Config{
		ProcessorSettings:      config.NewProcessorSettings(component.NewID(typeStr)),
		AggregationTemporality: "AGGREGATION_TEMPORALITY_CUMULATIVE",
		DimensionsCacheSize:    defaultDimensionsCacheSize,
		skipSanitizeLabel:      featuregate.GetRegistry().IsEnabled(dropSanitizationGateID),
	}
}

func createTracesProcessor(_ context.Context, params component.ProcessorCreateSettings, cfg component.ProcessorConfig, nextConsumer consumer.Traces) (component.TracesProcessor, error) {
	// TODO(srikanthccv): use the instanceID from params when it is added
	instanceUUID, _ := uuid.NewRandom()
	instanceID := instanceUUID.String()
	return newProcessor(params.Logger, instanceID, cfg, nextConsumer)
}

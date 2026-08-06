// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package signozspanmetricsprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestDeploymentEnvironmentDimensionResolution(t *testing.T) {
	makeAttrs := func(values map[string]string) pcommon.Map {
		attrs := pcommon.NewMap()
		for key, value := range values {
			attrs.PutStr(key, value)
		}
		return attrs
	}

	tests := []struct {
		name      string
		requested string
		span      map[string]string
		resource  map[string]string
		want      string
	}{
		{
			name:      "current only",
			requested: deploymentEnvironment,
			resource:  map[string]string{deploymentEnvironment: "production"},
			want:      "production",
		},
		{
			name:      "old only with current request",
			requested: deploymentEnvironment,
			resource:  map[string]string{deploymentEnvironmentOld: "production"},
			want:      "production",
		},
		{
			name:      "current only with old request",
			requested: deploymentEnvironmentOld,
			resource:  map[string]string{deploymentEnvironment: "production"},
			want:      "production",
		},
		{
			name:      "current wins conflict",
			requested: deploymentEnvironment,
			resource: map[string]string{
				deploymentEnvironment:    "production",
				deploymentEnvironmentOld: "staging",
			},
			want: "production",
		},
		{
			name:      "current resource wins old span",
			requested: deploymentEnvironment,
			span:      map[string]string{deploymentEnvironmentOld: "staging"},
			resource:  map[string]string{deploymentEnvironment: "production"},
			want:      "production",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value, ok := getDimensionValue(
				dimension{name: test.requested},
				makeAttrs(test.span),
				makeAttrs(test.resource),
			)
			require.True(t, ok)
			assert.Equal(t, test.want, value.Str())
		})
	}
}

func TestDeploymentEnvironmentDimensionTracksResourceOrigin(t *testing.T) {
	span := pcommon.NewMap()
	span.PutStr(deploymentEnvironment, "production")
	resource := pcommon.NewMap()
	resource.PutStr(deploymentEnvironmentOld, "staging")

	value, ok, foundInResource := getDimensionValueWithResource(
		dimension{name: deploymentEnvironment},
		span,
		resource,
	)
	require.True(t, ok)
	assert.True(t, foundInResource)
	assert.Equal(t, "production", value.Str())
}

func TestSpanmetricsEmitsCurrentDeploymentEnvironmentLabel(t *testing.T) {
	processor := &processorImp{
		attrsCardinality:                       map[string]map[string]struct{}{},
		serviceToOperations:                    map[string]map[string]struct{}{},
		maxNumberOfServicesToTrack:             100,
		maxNumberOfOperationsToTrackPerService: 100,
	}
	resource := pcommon.NewMap()
	resource.PutStr(deploymentEnvironmentOld, "production")

	dimensions := processor.buildDimensionKVs(
		"checkout",
		ptrace.NewSpan(),
		[]dimension{{name: deploymentEnvironment}},
		resource,
	)

	value, ok := dimensions.Get(deploymentEnvironment)
	require.True(t, ok)
	assert.Equal(t, "production", value.Str())
	_, oldExists := dimensions.Get(deploymentEnvironmentOld)
	assert.False(t, oldExists)
	resourceValue, ok := dimensions.Get(resourcePrefix + deploymentEnvironment)
	require.True(t, ok)
	assert.Equal(t, "production", resourceValue.Str())
}

func TestDBSystemDimensionResolution(t *testing.T) {
	span := pcommon.NewMap()
	span.PutStr(dbSystemOld, "mysql")
	span.PutStr(dbSystem, "postgresql")

	value, ok := getDimensionValue(dimension{name: dbSystem}, span, pcommon.NewMap())
	require.True(t, ok)
	assert.Equal(t, "postgresql", value.Str())

	legacyOnly := pcommon.NewMap()
	legacyOnly.PutStr(dbSystemOld, "redis")
	value, ok = getDimensionValue(dimension{name: dbSystem}, legacyOnly, pcommon.NewMap())
	require.True(t, ok)
	assert.Equal(t, "redis", value.Str())
}

func TestSpanmetricsEmitsCurrentDBSystemLabel(t *testing.T) {
	processor := &processorImp{
		attrsCardinality:                       map[string]map[string]struct{}{},
		serviceToOperations:                    map[string]map[string]struct{}{},
		maxNumberOfServicesToTrack:             100,
		maxNumberOfOperationsToTrackPerService: 100,
	}
	span := ptrace.NewSpan()
	span.Attributes().PutStr(dbSystemOld, "postgresql")

	dimensions := processor.buildCustomDimensionKVs(
		"checkout",
		span,
		[]dimension{{name: dbSystem}},
		pcommon.NewMap(),
		nil,
	)

	value, ok := dimensions.Get(dbSystem)
	require.True(t, ok)
	assert.Equal(t, "postgresql", value.Str())
	_, oldExists := dimensions.Get(dbSystemOld)
	assert.False(t, oldExists)
}

func TestDBClassificationAcceptsCurrentName(t *testing.T) {
	attributes := pcommon.NewMap()
	attributes.PutStr(dbSystem, "postgresql")
	value, ok := getFirstAttribute(attributes, dbSystem, dbSystemOld)
	require.True(t, ok)
	assert.Equal(t, "postgresql", value.Str())
}

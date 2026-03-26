package signozclickhousemetrics

import (
	"math"
	"testing"

	pkgfingerprint "github.com/SigNoz/signoz-otel-collector/internal/common/fingerprint"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap/zaptest"
)

const (
	baseTimestamp    = int64(1727286182000) // Base unix millisecond timestamp GMT: Wednesday, 25 September 2024 17:43:02
	timestampOffset1 = int64(1000)          // 1 second offset
	timestampOffset3 = int64(3000)          // 3 second offset
	timestampOffset4 = int64(4000)          // 4 second offset
	timestampOffset5 = int64(500)           // 0.5 second offset
	timestampOffset6 = int64(1500)          // 1.5 second offset
	unsetTimestamp   = int64(math.MaxInt64) // sentinel for "not provided"
)

type metadataCall struct {
	name        string
	desc        string
	unit        string
	typ         pmetric.MetricType
	temporality pmetric.AggregationTemporality
	isMonotonic bool
	fingerprint *pkgfingerprint.Fingerprint
	firstSeen   int64
	lastSeen    int64
}

type metadataCheck struct {
	seenKey                 string
	expectedMetricName      string
	expectedDescription     string
	expectedUnit            string
	expectedType            pmetric.MetricType
	expectedTemporality     pmetric.AggregationTemporality
	expectedAttrName        string
	expectedAttrType        string
	expectedAttrDatatype    pcommon.ValueType
	expectedAttrStringValue string
	expectedFirstReported   *int64
	expectedLastReported    *int64
}

type testCase struct {
	name              string
	setupAttrs        func() pcommon.Map
	fingerprintType   pkgfingerprint.FingerprintType
	calls             []metadataCall
	expectedMetaCount int
	checks            []metadataCheck
	customCheck       func(t *testing.T, b *batch)
}

func TestBatch_addMetadata(t *testing.T) {
	testCases := []testCase{
		{
			name: "add new metadata with timestamps",
			setupAttrs: func() pcommon.Map {
				attrs := pcommon.NewMap()
				attrs.PutStr("service.name", "test-service")
				attrs.PutStr("env", "prod")
				return attrs
			},
			fingerprintType: pkgfingerprint.PointFingerprintType,
			calls: []metadataCall{
				{
					name:        "test.metric",
					desc:        "Test metric description",
					unit:        "ms",
					typ:         pmetric.MetricTypeGauge,
					temporality: pmetric.AggregationTemporalityUnspecified,
					isMonotonic: false,
					firstSeen:   baseTimestamp,
					lastSeen:    baseTimestamp + timestampOffset1,
				},
			},
			expectedMetaCount: 2,
			checks: []metadataCheck{
				{
					seenKey:                 "service.nametest.metric",
					expectedMetricName:      "test.metric",
					expectedDescription:     "Test metric description",
					expectedUnit:            "ms",
					expectedType:            pmetric.MetricTypeGauge,
					expectedTemporality:     pmetric.AggregationTemporalityUnspecified,
					expectedAttrName:        "service.name",
					expectedAttrType:        "point",
					expectedAttrDatatype:    pcommon.ValueTypeStr,
					expectedAttrStringValue: "test-service",
					expectedFirstReported:   intPtr(baseTimestamp),
					expectedLastReported:    intPtr(baseTimestamp + timestampOffset1),
				},
				{
					seenKey:                 "envtest.metric",
					expectedAttrName:        "env",
					expectedAttrStringValue: "prod",
					expectedFirstReported:   intPtr(baseTimestamp),
					expectedLastReported:    intPtr(baseTimestamp + timestampOffset1),
				},
			},
		},
		{
			name: "add new metadata with nil timestamps",
			setupAttrs: func() pcommon.Map {
				attrs := pcommon.NewMap()
				attrs.PutStr("service.name", "test-service")
				return attrs
			},
			fingerprintType: pkgfingerprint.PointFingerprintType,
			calls: []metadataCall{
				{
					name:        "test.metric",
					desc:        "Test metric description",
					unit:        "ms",
					typ:         pmetric.MetricTypeGauge,
					temporality: pmetric.AggregationTemporalityUnspecified,
					isMonotonic: false,
					firstSeen:   unsetTimestamp,
					lastSeen:    unsetTimestamp,
				},
			},
			expectedMetaCount: 1,
			customCheck: func(t *testing.T, b *batch) {
				meta := b.metadata[0]
				assert.Greater(t, meta.firstReportedUnixMilli, int64(0))
				assert.Greater(t, meta.lastReportedUnixMilli, int64(0))
				assert.Equal(t, meta.firstReportedUnixMilli, meta.lastReportedUnixMilli, "first and last should be equal when unset")
			},
		},
		{
			name: "update existing metadata timestamps",
			setupAttrs: func() pcommon.Map {
				attrs := pcommon.NewMap()
				attrs.PutStr("service.name", "test-service")
				return attrs
			},
			fingerprintType: pkgfingerprint.PointFingerprintType,
			calls: []metadataCall{
				{
					name:        "test.metric",
					desc:        "Test metric description",
					unit:        "ms",
					typ:         pmetric.MetricTypeGauge,
					temporality: pmetric.AggregationTemporalityUnspecified,
					isMonotonic: false,
					firstSeen:   baseTimestamp,
					lastSeen:    baseTimestamp + timestampOffset1,
				},
				{
					name:        "test.metric",
					desc:        "Test metric description",
					unit:        "ms",
					typ:         pmetric.MetricTypeGauge,
					temporality: pmetric.AggregationTemporalityUnspecified,
					isMonotonic: false,
					firstSeen:   baseTimestamp - timestampOffset5,
					lastSeen:    baseTimestamp + timestampOffset3,
				},
			},
			expectedMetaCount: 1,
			checks: []metadataCheck{
				{
					seenKey:               "service.nametest.metric",
					expectedFirstReported: intPtr(baseTimestamp - timestampOffset5),
					expectedLastReported:  intPtr(baseTimestamp + timestampOffset3),
				},
			},
		},
		{
			name: "update existing metadata with nil timestamps updates with current time",
			setupAttrs: func() pcommon.Map {
				attrs := pcommon.NewMap()
				attrs.PutStr("service.name", "test-service")
				return attrs
			},
			fingerprintType: pkgfingerprint.PointFingerprintType,
			calls: []metadataCall{
				{
					name:        "test.metric",
					desc:        "Test metric description",
					unit:        "ms",
					typ:         pmetric.MetricTypeGauge,
					temporality: pmetric.AggregationTemporalityUnspecified,
					isMonotonic: false,
					firstSeen:   baseTimestamp,
					lastSeen:    baseTimestamp + timestampOffset1,
				},
				{
					name:        "test.metric",
					desc:        "Test metric description",
					unit:        "ms",
					typ:         pmetric.MetricTypeGauge,
					temporality: pmetric.AggregationTemporalityUnspecified,
					isMonotonic: false,
					firstSeen:   unsetTimestamp,
					lastSeen:    unsetTimestamp,
				},
			},
			expectedMetaCount: 1,
			customCheck: func(t *testing.T, b *batch) {
				idx, exists := b.metaIdx["service.nametest.metric"]
				require.True(t, exists)
				meta := b.metadata[idx]
				// firstReportedUnixMilli should remain at baseTimestamp because time.Now() > baseTimestamp,
				// so the update logic (which only updates if new < existing) won't update it
				assert.Equal(t, baseTimestamp, meta.firstReportedUnixMilli)
				// lastReportedUnixMilli should be updated to time.Now() because time.Now() > existing last
				assert.Greater(t, meta.lastReportedUnixMilli, baseTimestamp+timestampOffset1)
				// First should be <= last
				assert.LessOrEqual(t, meta.firstReportedUnixMilli, meta.lastReportedUnixMilli)
			},
		},
		{
			name: "special handling for le key",
			setupAttrs: func() pcommon.Map {
				attrs := pcommon.NewMap()
				attrs.PutStr("le", "0.5")
				attrs.PutStr("service.name", "test-service")
				return attrs
			},
			fingerprintType: pkgfingerprint.PointFingerprintType,
			calls: []metadataCall{
				{
					name:        "test.metric",
					desc:        "Test metric description",
					unit:        "ms",
					typ:         pmetric.MetricTypeHistogram,
					temporality: pmetric.AggregationTemporalityCumulative,
					isMonotonic: false,
					firstSeen:   baseTimestamp,
					lastSeen:    baseTimestamp + timestampOffset1,
				},
			},
			expectedMetaCount: 2,
			checks: []metadataCheck{
				{
					seenKey:                 "letest.metric0.5",
					expectedAttrName:        "le",
					expectedAttrStringValue: "0.5",
				},
				{
					seenKey:          "service.nametest.metric",
					expectedAttrName: "service.name",
				},
			},
		},
		{
			name: "update timestamps when firstReportedUnixMilli is zero",
			setupAttrs: func() pcommon.Map {
				attrs := pcommon.NewMap()
				attrs.PutStr("service.name", "test-service")
				return attrs
			},
			fingerprintType: pkgfingerprint.PointFingerprintType,
			calls: []metadataCall{
				{
					name:        "test.metric",
					desc:        "Test metric description",
					unit:        "ms",
					typ:         pmetric.MetricTypeGauge,
					temporality: pmetric.AggregationTemporalityUnspecified,
					isMonotonic: false,
					firstSeen:   0,
					lastSeen:    0,
				},
				{
					name:        "test.metric",
					desc:        "Test metric description",
					unit:        "ms",
					typ:         pmetric.MetricTypeGauge,
					temporality: pmetric.AggregationTemporalityUnspecified,
					isMonotonic: false,
					firstSeen:   baseTimestamp,
					lastSeen:    baseTimestamp + timestampOffset1,
				},
			},
			expectedMetaCount: 1,
			checks: []metadataCheck{
				{
					seenKey:               "service.nametest.metric",
					expectedFirstReported: intPtr(baseTimestamp),
					expectedLastReported:  intPtr(baseTimestamp + timestampOffset1),
				},
			},
		},
		{
			name: "multiple attributes with different metric names",
			setupAttrs: func() pcommon.Map {
				attrs := pcommon.NewMap()
				attrs.PutStr("service.name", "test-service")
				attrs.PutStr("env", "prod")
				return attrs
			},
			fingerprintType: pkgfingerprint.PointFingerprintType,
			calls: []metadataCall{
				{
					name:        "metric1",
					desc:        "Metric 1 description",
					unit:        "ms",
					typ:         pmetric.MetricTypeGauge,
					temporality: pmetric.AggregationTemporalityUnspecified,
					isMonotonic: false,
					firstSeen:   baseTimestamp,
					lastSeen:    baseTimestamp + timestampOffset1,
				},
				{
					name:        "metric2",
					desc:        "Metric 2 description",
					unit:        "s",
					typ:         pmetric.MetricTypeSum,
					temporality: pmetric.AggregationTemporalityCumulative,
					isMonotonic: true,
					firstSeen:   baseTimestamp + timestampOffset3,
					lastSeen:    baseTimestamp + timestampOffset4,
				},
			},
			expectedMetaCount: 4,
			checks: []metadataCheck{
				{
					seenKey:               "service.namemetric1",
					expectedMetricName:    "metric1",
					expectedFirstReported: intPtr(baseTimestamp),
				},
				{
					seenKey:               "service.namemetric2",
					expectedMetricName:    "metric2",
					expectedFirstReported: intPtr(baseTimestamp + timestampOffset3),
				},
			},
		},
		{
			name: "update timestamps only when new first is earlier or new last is later",
			setupAttrs: func() pcommon.Map {
				attrs := pcommon.NewMap()
				attrs.PutStr("service.name", "test-service")
				return attrs
			},
			fingerprintType: pkgfingerprint.PointFingerprintType,
			calls: []metadataCall{
				{
					name:        "test.metric",
					desc:        "Test metric description",
					unit:        "ms",
					typ:         pmetric.MetricTypeGauge,
					temporality: pmetric.AggregationTemporalityUnspecified,
					isMonotonic: false,
					firstSeen:   baseTimestamp,
					lastSeen:    baseTimestamp + timestampOffset1,
				},
				{
					name:        "test.metric",
					desc:        "Test metric description",
					unit:        "ms",
					typ:         pmetric.MetricTypeGauge,
					temporality: pmetric.AggregationTemporalityUnspecified,
					isMonotonic: false,
					firstSeen:   baseTimestamp + timestampOffset6,
					lastSeen:    baseTimestamp + timestampOffset5,
				},
			},
			expectedMetaCount: 1,
			checks: []metadataCheck{
				{
					seenKey:               "service.nametest.metric",
					expectedFirstReported: intPtr(baseTimestamp),
					expectedLastReported:  intPtr(baseTimestamp + timestampOffset1),
				},
			},
		},
		{
			name: "resource fingerprint type",
			setupAttrs: func() pcommon.Map {
				attrs := pcommon.NewMap()
				attrs.PutStr("service.name", "test-service")
				return attrs
			},
			fingerprintType: pkgfingerprint.ResourceFingerprintType,
			calls: []metadataCall{
				{
					name:        "test.metric",
					desc:        "Test metric description",
					unit:        "ms",
					typ:         pmetric.MetricTypeGauge,
					temporality: pmetric.AggregationTemporalityUnspecified,
					isMonotonic: false,
					firstSeen:   baseTimestamp,
					lastSeen:    baseTimestamp + timestampOffset1,
				},
			},
			expectedMetaCount: 1,
			checks: []metadataCheck{
				{
					seenKey:          "service.nametest.metric",
					expectedAttrType: "resource",
				},
			},
		},
		{
			name: "scope fingerprint type",
			setupAttrs: func() pcommon.Map {
				attrs := pcommon.NewMap()
				attrs.PutStr("scope.name", "test-scope")
				return attrs
			},
			fingerprintType: pkgfingerprint.ScopeFingerprintType,
			calls: []metadataCall{
				{
					name:        "test.metric",
					desc:        "Test metric description",
					unit:        "ms",
					typ:         pmetric.MetricTypeGauge,
					temporality: pmetric.AggregationTemporalityUnspecified,
					isMonotonic: false,
					firstSeen:   baseTimestamp,
					lastSeen:    baseTimestamp + timestampOffset1,
				},
			},
			expectedMetaCount: 1,
			checks: []metadataCheck{
				{
					seenKey:          "scope.nametest.metric",
					expectedAttrType: "scope",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			b := newBatch(zaptest.NewLogger(t))
			attrs := tc.setupAttrs()
			fp := pkgfingerprint.NewFingerprint(tc.fingerprintType, 0, attrs, nil)

			for _, call := range tc.calls {
				call.fingerprint = fp
				b.addMetadata(
					call.name,
					call.desc,
					call.unit,
					call.typ,
					call.temporality,
					call.isMonotonic,
					call.fingerprint,
					call.firstSeen,
					call.lastSeen,
				)
			}

			require.Equal(t, tc.expectedMetaCount, len(b.metadata), "metadata count mismatch")
			require.Equal(t, tc.expectedMetaCount, len(b.metaIdx), "metaIdx count mismatch")
			require.Equal(t, tc.expectedMetaCount, len(b.metaSeen), "metaSeen count mismatch")

			for _, check := range tc.checks {
				idx, exists := b.metaIdx[check.seenKey]
				require.True(t, exists, "should have entry for seenKey: %s", check.seenKey)
				meta := b.metadata[idx]

				if check.expectedMetricName != "" {
					assert.Equal(t, check.expectedMetricName, meta.metricName, "metric name mismatch for %s", check.seenKey)
				}
				if check.expectedDescription != "" {
					assert.Equal(t, check.expectedDescription, meta.description, "description mismatch for %s", check.seenKey)
				}
				if check.expectedUnit != "" {
					assert.Equal(t, check.expectedUnit, meta.unit, "unit mismatch for %s", check.seenKey)
				}
				if check.expectedType != pmetric.MetricTypeEmpty {
					assert.Equal(t, check.expectedType, meta.typ, "type mismatch for %s", check.seenKey)
				}
				// Check temporality if it's not the default unspecified value or if explicitly set in first test case
				if check.expectedTemporality != pmetric.AggregationTemporalityUnspecified {
					assert.Equal(t, check.expectedTemporality, meta.temporality, "temporality mismatch for %s", check.seenKey)
				}
				// Check isMonotonic by finding the matching call
				if len(tc.calls) > 0 {
					for _, call := range tc.calls {
						if call.name == meta.metricName {
							assert.Equal(t, call.isMonotonic, meta.isMonotonic, "isMonotonic mismatch for %s", check.seenKey)
							break
						}
					}
				}
				if check.expectedAttrName != "" {
					assert.Equal(t, check.expectedAttrName, meta.attrName, "attrName mismatch for %s", check.seenKey)
				}
				if check.expectedAttrType != "" {
					assert.Equal(t, check.expectedAttrType, meta.attrType, "attrType mismatch for %s", check.seenKey)
				}
				if check.expectedAttrDatatype != pcommon.ValueTypeEmpty {
					assert.Equal(t, check.expectedAttrDatatype, meta.attrDatatype, "attrDatatype mismatch for %s", check.seenKey)
				}
				if check.expectedAttrStringValue != "" {
					assert.Equal(t, check.expectedAttrStringValue, meta.attrStringValue, "attrStringValue mismatch for %s", check.seenKey)
				}
				if check.expectedFirstReported != nil {
					assert.Equal(t, *check.expectedFirstReported, meta.firstReportedUnixMilli, "firstReportedUnixMilli mismatch for %s", check.seenKey)
				}
				if check.expectedLastReported != nil {
					assert.Equal(t, *check.expectedLastReported, meta.lastReportedUnixMilli, "lastReportedUnixMilli mismatch for %s", check.seenKey)
				}
			}

			if tc.customCheck != nil {
				tc.customCheck(t, b)
			}
		})
	}
}

func intPtr(i int64) *int64 {
	return &i
}

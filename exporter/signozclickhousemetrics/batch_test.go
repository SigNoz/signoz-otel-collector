package signozclickhousemetrics

import (
	"math"
	"testing"
	"time"

	pkgfingerprint "github.com/SigNoz/signoz-otel-collector/internal/common/fingerprint"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap/zaptest"
)

func TestBatch_addMetadata(t *testing.T) {

	var (
		baseTimestamp  = time.Now().UnixMilli() - (24 * 60 * 60 * 1000) // now() - 1 day
		unsetTimestamp = int64(math.MaxInt64)                           // sentinel for "not provided"
	)

	tests := []struct {
		name            string
		metricName      string
		metricDesc      string
		metricUnit      string
		metricType      pmetric.MetricType
		temporality     pmetric.AggregationTemporality
		isMonotonic     bool
		setupAttrs      func() pcommon.Map
		fingerprintType pkgfingerprint.FingerprintType
		firstSeen       int64
		lastSeen        int64
		validate        func(t *testing.T, b *batch)
	}{
		{
			name:        "default timestamps when MaxInt64",
			metricName:  "test.metric",
			metricDesc:  "Test description",
			metricUnit:  "ms",
			metricType:  pmetric.MetricTypeGauge,
			temporality: pmetric.AggregationTemporalityUnspecified,
			isMonotonic: false,
			setupAttrs: func() pcommon.Map {
				attrs := pcommon.NewMap()
				attrs.PutStr("service.name", "test-service")
				return attrs
			},
			fingerprintType: pkgfingerprint.PointFingerprintType,
			firstSeen:       unsetTimestamp,
			lastSeen:        unsetTimestamp,
			validate: func(t *testing.T, b *batch) {
				require.Len(t, b.metadata, 1)
				m := b.metadata[0]

				// Timestamps should be set to now
				assert.Greater(t, m.firstReportedUnixMilli, baseTimestamp)
				assert.Less(t, m.firstReportedUnixMilli, time.Now().UnixMilli()+5000)
				assert.Equal(t, m.firstReportedUnixMilli, m.lastReportedUnixMilli, "first and last should be equal when defaulted")

				// Metric properties
				assert.Equal(t, "test.metric", m.metricName)
				assert.Equal(t, "Test description", m.description)
				assert.Equal(t, "ms", m.unit)
				assert.Equal(t, pmetric.MetricTypeGauge, m.typ)
				assert.Equal(t, pmetric.AggregationTemporalityUnspecified, m.temporality)
				assert.False(t, m.isMonotonic)

				// Attribute properties
				assert.Equal(t, "service.name", m.attrName)
				assert.Equal(t, "test-service", m.attrStringValue)
				assert.Equal(t, "point", m.attrType)
				assert.Equal(t, pcommon.ValueTypeStr, m.attrDatatype)
			},
		},
		{
			name:        "normal timestamps preserved",
			metricName:  "test.metric",
			metricDesc:  "Test description",
			metricUnit:  "ms",
			metricType:  pmetric.MetricTypeGauge,
			temporality: pmetric.AggregationTemporalityUnspecified,
			isMonotonic: false,
			setupAttrs: func() pcommon.Map {
				attrs := pcommon.NewMap()
				attrs.PutStr("service.name", "test-service")
				return attrs
			},
			fingerprintType: pkgfingerprint.PointFingerprintType,
			firstSeen:       baseTimestamp,
			lastSeen:        baseTimestamp + 1000,
			validate: func(t *testing.T, b *batch) {
				require.Len(t, b.metadata, 1)
				m := b.metadata[0]

				// Timestamps should match exactly
				assert.Equal(t, baseTimestamp, m.firstReportedUnixMilli)
				assert.Equal(t, baseTimestamp+1000, m.lastReportedUnixMilli)

				// Basic properties
				assert.Equal(t, "test.metric", m.metricName)
				assert.Equal(t, "service.name", m.attrName)
			},
		},
		{
			name:        "all attributes added to metadata",
			metricName:  "test.metric",
			metricDesc:  "Test description",
			metricUnit:  "ms",
			metricType:  pmetric.MetricTypeGauge,
			temporality: pmetric.AggregationTemporalityUnspecified,
			isMonotonic: false,
			setupAttrs: func() pcommon.Map {
				attrs := pcommon.NewMap()
				attrs.PutStr("service.name", "test-service")
				attrs.PutStr("env", "prod")
				attrs.PutStr("host", "host1")
				attrs.PutStr("region", "us-west")
				return attrs
			},
			fingerprintType: pkgfingerprint.PointFingerprintType,
			firstSeen:       baseTimestamp,
			lastSeen:        baseTimestamp + 1000,
			validate: func(t *testing.T, b *batch) {
				require.Len(t, b.metadata, 4, "should have one entry per attribute")

				// Collect all attribute names and values
				attrMap := make(map[string]string)
				for _, m := range b.metadata {
					// All should have same metric properties
					assert.Equal(t, "test.metric", m.metricName)
					assert.Equal(t, baseTimestamp, m.firstReportedUnixMilli)
					assert.Equal(t, baseTimestamp+1000, m.lastReportedUnixMilli)
					assert.Equal(t, "point", m.attrType)

					attrMap[m.attrName] = m.attrStringValue
				}

				// Verify all attributes are present
				assert.Equal(t, "test-service", attrMap["service.name"])
				assert.Equal(t, "prod", attrMap["env"])
				assert.Equal(t, "host1", attrMap["host"])
				assert.Equal(t, "us-west", attrMap["region"])
			},
		},
		{
			name:        "point fingerprint type",
			metricName:  "test.metric",
			metricDesc:  "Test description",
			metricUnit:  "ms",
			metricType:  pmetric.MetricTypeGauge,
			temporality: pmetric.AggregationTemporalityUnspecified,
			isMonotonic: false,
			setupAttrs: func() pcommon.Map {
				attrs := pcommon.NewMap()
				attrs.PutStr("service.name", "test-service")
				return attrs
			},
			fingerprintType: pkgfingerprint.PointFingerprintType,
			firstSeen:       baseTimestamp,
			lastSeen:        baseTimestamp + 1000,
			validate: func(t *testing.T, b *batch) {
				require.Len(t, b.metadata, 1)
				assert.Equal(t, "point", b.metadata[0].attrType)
			},
		},
		{
			name:        "resource fingerprint type",
			metricName:  "test.metric",
			metricDesc:  "Test description",
			metricUnit:  "ms",
			metricType:  pmetric.MetricTypeGauge,
			temporality: pmetric.AggregationTemporalityUnspecified,
			isMonotonic: false,
			setupAttrs: func() pcommon.Map {
				attrs := pcommon.NewMap()
				attrs.PutStr("service.name", "test-service")
				return attrs
			},
			fingerprintType: pkgfingerprint.ResourceFingerprintType,
			firstSeen:       baseTimestamp,
			lastSeen:        baseTimestamp + 1000,
			validate: func(t *testing.T, b *batch) {
				require.Len(t, b.metadata, 1)
				assert.Equal(t, "resource", b.metadata[0].attrType)
			},
		},
		{
			name:        "scope fingerprint type",
			metricName:  "test.metric",
			metricDesc:  "Test description",
			metricUnit:  "ms",
			metricType:  pmetric.MetricTypeGauge,
			temporality: pmetric.AggregationTemporalityUnspecified,
			isMonotonic: false,
			setupAttrs: func() pcommon.Map {
				attrs := pcommon.NewMap()
				attrs.PutStr("scope.name", "test-scope")
				return attrs
			},
			fingerprintType: pkgfingerprint.ScopeFingerprintType,
			firstSeen:       baseTimestamp,
			lastSeen:        baseTimestamp + 1000,
			validate: func(t *testing.T, b *batch) {
				require.Len(t, b.metadata, 1)
				assert.Equal(t, "scope", b.metadata[0].attrType)
			},
		},
		{
			name:        "multiple calls append without deduplication",
			metricName:  "test.metric",
			metricDesc:  "Test description",
			metricUnit:  "ms",
			metricType:  pmetric.MetricTypeGauge,
			temporality: pmetric.AggregationTemporalityUnspecified,
			isMonotonic: false,
			setupAttrs: func() pcommon.Map {
				attrs := pcommon.NewMap()
				attrs.PutStr("service.name", "test-service")
				attrs.PutStr("env", "prod")
				return attrs
			},
			fingerprintType: pkgfingerprint.PointFingerprintType,
			firstSeen:       baseTimestamp,
			lastSeen:        baseTimestamp + 1000,
			validate: func(t *testing.T, b *batch) {
				// First call adds 2 entries
				require.Len(t, b.metadata, 2)

				// Second call with same attributes
				attrs := pcommon.NewMap()
				attrs.PutStr("service.name", "test-service")
				attrs.PutStr("env", "prod")
				fp := pkgfingerprint.NewFingerprint(pkgfingerprint.PointFingerprintType, 0, attrs, nil)
				b.addMetadata("test.metric", "Test description", "ms",
					pmetric.MetricTypeGauge, pmetric.AggregationTemporalityUnspecified,
					false, fp, baseTimestamp, baseTimestamp+1000)

				// Should have 4 entries total (no deduplication)
				require.Len(t, b.metadata, 4)

				// Verify all entries
				count := 0
				for _, m := range b.metadata {
					if m.attrName == "service.name" || m.attrName == "env" {
						count++
					}
				}
				assert.Equal(t, 4, count, "should have 4 entries total")
			},
		},
		{
			name:        "various attribute data types",
			metricName:  "test.metric",
			metricDesc:  "Test description",
			metricUnit:  "ms",
			metricType:  pmetric.MetricTypeGauge,
			temporality: pmetric.AggregationTemporalityUnspecified,
			isMonotonic: false,
			setupAttrs: func() pcommon.Map {
				attrs := pcommon.NewMap()
				attrs.PutStr("str_attr", "value")
				attrs.PutInt("int_attr", 42)
				attrs.PutBool("bool_attr", true)
				attrs.PutDouble("double_attr", 3.14)
				return attrs
			},
			fingerprintType: pkgfingerprint.PointFingerprintType,
			firstSeen:       baseTimestamp,
			lastSeen:        baseTimestamp + 1000,
			validate: func(t *testing.T, b *batch) {
				require.Len(t, b.metadata, 4)

				// Create map for easy lookup
				attrTypeMap := make(map[string]pcommon.ValueType)
				for _, m := range b.metadata {
					attrTypeMap[m.attrName] = m.attrDatatype
				}

				assert.Equal(t, pcommon.ValueTypeStr, attrTypeMap["str_attr"])
				assert.Equal(t, pcommon.ValueTypeInt, attrTypeMap["int_attr"])
				assert.Equal(t, pcommon.ValueTypeBool, attrTypeMap["bool_attr"])
				assert.Equal(t, pcommon.ValueTypeDouble, attrTypeMap["double_attr"])
			},
		},
		{
			name:        "empty fingerprint attributes",
			metricName:  "test.metric",
			metricDesc:  "Test description",
			metricUnit:  "ms",
			metricType:  pmetric.MetricTypeGauge,
			temporality: pmetric.AggregationTemporalityUnspecified,
			isMonotonic: false,
			setupAttrs: func() pcommon.Map {
				return pcommon.NewMap() // empty attributes
			},
			fingerprintType: pkgfingerprint.PointFingerprintType,
			firstSeen:       baseTimestamp,
			lastSeen:        baseTimestamp + 1000,
			validate: func(t *testing.T, b *batch) {
				require.Len(t, b.metadata, 0, "should have no metadata entries for empty attributes")
			},
		},
		{
			name:        "metric type sum with monotonic",
			metricName:  "test.counter",
			metricDesc:  "Test counter",
			metricUnit:  "1",
			metricType:  pmetric.MetricTypeSum,
			temporality: pmetric.AggregationTemporalityCumulative,
			isMonotonic: true,
			setupAttrs: func() pcommon.Map {
				attrs := pcommon.NewMap()
				attrs.PutStr("service.name", "test-service")
				return attrs
			},
			fingerprintType: pkgfingerprint.PointFingerprintType,
			firstSeen:       baseTimestamp,
			lastSeen:        baseTimestamp + 1000,
			validate: func(t *testing.T, b *batch) {
				require.Len(t, b.metadata, 1)
				m := b.metadata[0]

				assert.Equal(t, pmetric.MetricTypeSum, m.typ)
				assert.Equal(t, pmetric.AggregationTemporalityCumulative, m.temporality)
				assert.True(t, m.isMonotonic)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := newBatch(zaptest.NewLogger(t))
			attrs := tt.setupAttrs()
			fp := pkgfingerprint.NewFingerprint(tt.fingerprintType, 0, attrs, nil)

			b.addMetadata(tt.metricName, tt.metricDesc, tt.metricUnit,
				tt.metricType, tt.temporality, tt.isMonotonic,
				fp, tt.firstSeen, tt.lastSeen)

			tt.validate(t, b)
		})
	}
}

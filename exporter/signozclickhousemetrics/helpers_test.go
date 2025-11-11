package signozclickhousemetrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	semconv "go.opentelemetry.io/collector/semconv/v1.16.0"
)

func TestBucketForMetricDelay(t *testing.T) {
	tests := []struct {
		name         string
		delay        time.Duration
		expectBucket string
		expectFound  bool
	}{
		{name: "within 5m-10m", delay: 6 * time.Minute, expectBucket: "5m-10m", expectFound: true},
		{name: "within 10m-30m", delay: 20 * time.Minute, expectBucket: "10m-30m", expectFound: true},
		{name: "boundary at 30m", delay: 30 * time.Minute, expectBucket: "30m-2h", expectFound: true},
		{name: "within 2h-6h", delay: 3 * time.Hour, expectBucket: "2h-6h", expectFound: true},
		{name: "within 6h-1d", delay: 18 * time.Hour, expectBucket: "6h-1d", expectFound: true},
		{name: "below minimum", delay: 4 * time.Minute, expectBucket: "", expectFound: false},
		{name: "above maximum", delay: 36 * time.Hour, expectBucket: "", expectFound: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bucket, ok := bucketForMetricDelay(tt.delay)
			require.Equal(t, tt.expectFound, ok)
			if !tt.expectFound {
				require.Equal(t, bucketDefinition{}, bucket)
				return
			}
			require.Equal(t, tt.expectBucket, bucket.name)
		})
	}
}

func TestExtractPrimaryMetricLabels(t *testing.T) {
	tests := []struct {
		name           string
		resourceMap    map[string]string
		fingerprintMap map[string]string
		expected       map[string]string
	}{
		{
			name: "all attributes present",
			resourceMap: map[string]string{
				string(semconv.AttributeServiceName):   "orders-service",
				semconv.AttributeServiceNamespace:      "ecommerce",
				semconv.AttributeDeploymentEnvironment: "prod",
			},
			fingerprintMap: map[string]string{
				"span.kind": "server",
				"operation": "GET /orders",
			},
			expected: map[string]string{
				"service.name":           "orders-service",
				"service.namespace":      "ecommerce",
				"deployment.environment": "prod",
				"span.kind":              "server",
				"operation":              "GET /orders",
			},
		},
		{
			name: "missing optional attributes",
			resourceMap: map[string]string{
				string(semconv.AttributeServiceName): "billing-service",
			},
			fingerprintMap: map[string]string{
				"operation": "POST /charge",
			},
			expected: map[string]string{
				"service.name": "billing-service",
				"operation":    "POST /charge",
			},
		},
		{
			name:           "no attributes present",
			resourceMap:    map[string]string{},
			fingerprintMap: map[string]string{},
			expected:       map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPrimaryMetricLabels(tt.resourceMap, tt.fingerprintMap)
			require.Equal(t, tt.expected, got)
		})
	}
}

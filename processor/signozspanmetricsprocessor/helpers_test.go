package signozspanmetricsprocessor

import (
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
)

func TestExtractSpanResourceInfo(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() pcommon.Map
		expected map[string]string
	}{
		{
			name: "all keys present",
			setup: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr(string(conventions.AttributeServiceNamespace), "payments")
				m.PutStr(string(conventions.AttributeDeploymentEnvironment), "prod")
				m.PutStr(signozID, "instance-1")
				return m
			},
			expected: map[string]string{
				string(conventions.AttributeServiceNamespace):      "payments",
				string(conventions.AttributeDeploymentEnvironment): "prod",
				signozID: "instance-1",
			},
		},
		{
			name: "partial keys present",
			setup: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr(string(conventions.AttributeDeploymentEnvironment), "staging")
				return m
			},
			expected: map[string]string{
				string(conventions.AttributeDeploymentEnvironment): "staging",
			},
		},
		{
			name: "no keys present",
			setup: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("unrelated", "value")
				return m
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resourceMap := tt.setup()
			got := extractSpanResourceInfo(resourceMap)

			if len(tt.expected) == 0 {
				if got != nil {
					t.Fatalf("expected nil, got %v", got)
				}
				return
			}

			if len(got) != len(tt.expected) {
				t.Fatalf("expected %d keys, got %d", len(tt.expected), len(got))
			}
			for k, v := range tt.expected {
				if got[k] != v {
					t.Errorf("expected %s=%s, got %s", k, v, got[k])
				}
			}
		})
	}
}

func TestBucketForSpanDelay(t *testing.T) {
	tests := []struct {
		name           string
		delay          time.Duration
		expectedBucket string
		expectedOK     bool
	}{
		{"within first bucket", 7 * time.Minute, "5m-10m", true},
		{"within middle bucket", 25 * time.Minute, "10m-30m", true},
		{"within last bucket", 12 * time.Hour, "6h-1d", true},
		{"below minimum", 4 * time.Minute, "", false},
		{"above maximum", 30 * time.Hour, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bucket, ok := bucketForSpanDelay(tt.delay)
			if ok != tt.expectedOK {
				t.Fatalf("expected ok=%v, got %v", tt.expectedOK, ok)
			}
			if !tt.expectedOK {
				return
			}
			if bucket.name != tt.expectedBucket {
				t.Errorf("expected bucket %q, got %q", tt.expectedBucket, bucket.name)
			}
		})
	}
}

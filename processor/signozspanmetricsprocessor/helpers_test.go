package signozspanmetricsprocessor

import (
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
)

func TestCollectAndResetLateSpanData(t *testing.T) {
	logger := zap.NewNop()
	p := &processorImp{
		logger:       logger,
		lateSpanData: make(map[string]*lateSpanBucketStats),
	}

	// Seed bucket with synthetic data.
	p.lateSpanData["5m-10m"] = &lateSpanBucketStats{
		Count:     2,
		FirstSpan: spanSummary{Service: "svc-a", SpanName: "span-1", DelaySeconds: 360},
		MinDelay:  6 * time.Minute,
		MinSpan:   spanSummary{Service: "svc-b", SpanName: "span-2", DelaySeconds: 360},
		MaxDelay:  9 * time.Minute,
		MaxSpan:   spanSummary{Service: "svc-a", SpanName: "span-3", DelaySeconds: 540},
		ServiceMap: map[string]*serviceSample{
			"svc-a": {Count: 1, SampleSpanNames: []string{"span-1"}},
			"svc-b": {Count: 1, SampleSpanNames: []string{"span-2"}},
		},
	}

	reports := p.collectAndResetLateSpanData()
	if len(reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(reports))
	}

	if reports[0].Bucket != "5m-10m" {
		t.Errorf("expected bucket '5m-10m', got %q", reports[0].Bucket)
	}
	if reports[0].Count != 2 {
		t.Errorf("expected count 2, got %d", reports[0].Count)
	}
	if reports[0].MinDelaySeconds != 360 {
		t.Errorf("expected min delay seconds 360, got %d", reports[0].MinDelaySeconds)
	}
	if reports[0].MaxDelaySeconds != 540 {
		t.Errorf("expected max delay seconds 540, got %d", reports[0].MaxDelaySeconds)
	}

	if _, ok := reports[0].ServiceStats["svc-a"]; !ok {
		t.Fatalf("expected service svc-a in report")
	}
	if reports[0].ServiceStats["svc-a"].Count != 1 {
		t.Errorf("expected svc-a count 1, got %d", reports[0].ServiceStats["svc-a"].Count)
	}

	// Map should be reset after collection.
	if len(p.lateSpanData) != 0 {
		t.Fatalf("expected lateSpanData to be reset, found %d buckets", len(p.lateSpanData))
	}
}

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

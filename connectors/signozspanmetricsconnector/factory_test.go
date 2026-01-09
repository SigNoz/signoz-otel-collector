package signozspanmetricsconnector

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/SigNoz/signoz-otel-collector/connectors/signozspanmetricsconnector/internal/metadata"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestNewConnector(t *testing.T) {
	defaultMethod := http.MethodGet
	defaultMethodValue := pcommon.NewValueStr(defaultMethod)
	for _, tc := range []struct {
		name                         string
		durationHistogramBuckets     []time.Duration
		dimensions                   []Dimension
		wantDurationHistogramBuckets []float64
		wantDimensions               []dimension
	}{
		{
			name: "simplest config (use defaults)",
		},
		{
			name:                     "1 configured duration histogram bucket should result in 1 explicit duration bucket (+1 implicit +Inf bucket)",
			durationHistogramBuckets: []time.Duration{2 * time.Millisecond},
			dimensions: []Dimension{
				{Name: "http.method", Default: &defaultMethod},
				{Name: "http.status_code"},
			},
			wantDimensions: []dimension{
				{name: "http.method", value: &defaultMethodValue},
				{name: "http.status_code", value: nil},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Prepare
			factory := NewFactory()

			creationParams := connectortest.NewNopSettings(metadata.Type)
			creationParams.Resource.Attributes().PutStr(collectorInstanceKey, uuid.NewString())
			cfg := factory.CreateDefaultConfig().(*Config)
			cfg.Dimensions = tc.dimensions

			// Test
			traceConnector, err := factory.CreateTracesToMetrics(context.Background(), creationParams, cfg, consumertest.NewNop())
			smc := traceConnector.(*connectorImp)

			// Verify
			assert.NoError(t, err)
			assert.NotNil(t, smc)

			assert.Equal(t, tc.wantDimensions, smc.dimensions)
		})
	}
}

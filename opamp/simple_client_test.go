package opamp

import (
	"context"
	"testing"

	"github.com/SigNoz/signoz-otel-collector/signozcol"
	"go.opentelemetry.io/collector/service"
	"go.uber.org/zap"
)

func TestNopClientWithCollector(t *testing.T) {
	coll := signozcol.New(signozcol.WrappedCollectorSettings{
		ConfigPaths: []string{"testdata/config.yaml"},
		Version:     "0.0.1",
		Desc:        "test",
		LoggingOpts: []zap.Option{zap.AddStacktrace(zap.ErrorLevel)},
	})

	client := NewSimpleClient(coll)

	err := client.Start(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if coll.GetState() != service.StateRunning {
		t.Errorf("expected collector to be run")
	}

	err = client.Stop(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

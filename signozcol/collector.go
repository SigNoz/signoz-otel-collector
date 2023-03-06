package signozcol

import (
	"context"
	"sync"

	"go.opentelemetry.io/collector/service"
	"go.uber.org/zap"
)

// WrappedCollector is a wrapper around the OpenTelemetry Collector
// that allows it to be started and stopped.
// It internally uses the OpenTelemetry Collector's service package.

// On restart, the collector is stopped and a new instance is started.
// The Opamp client implementation is responsible for restarting the collector.
type WrappedCollector struct {
	configPaths []string
	version     string
	desc        string
	loggingOpts []zap.Option
	mux         sync.Mutex
	svc         *service.Collector
}

type WrappedCollectorSettings struct {
	ConfigPaths []string
	Version     string
	Desc        string
	LoggingOpts []zap.Option
}

// Run runs the collector.
func (c *WrappedCollector) Run(ctx context.Context) error {
	return nil
}

// Shutdown stops the collector.
func (c *WrappedCollector) Shutdown() {

}

// Restart restarts the collector.
func (c *WrappedCollector) Restart(ctx context.Context) error {
	return nil
}

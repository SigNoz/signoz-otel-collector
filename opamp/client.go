package opamp

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SigNoz/signoz-otel-collector/signozcol"
	"go.opentelemetry.io/collector/otelcol"
	"go.uber.org/zap"
)

type Client interface {
	Start(ctx context.Context) error

	Stop(ctx context.Context) error

	Error() <-chan error
}

type baseClient struct {
	err     chan error
	stopped chan bool
	coll    *signozcol.WrappedCollector
	logger  *zap.Logger

	reloadMux      sync.Mutex
	isReloading    atomic.Bool
	lastKnownState atomic.Int32
}

// Error returns the error channel
func (c *baseClient) Error() <-chan error {
	return c.err
}

// ensureRunning checks if the collector is running
// and sends an error to the error channel if it is not
// running
//
// The error channel is used to signal the main function
// to shutdown the service
//
// The collector may stop running unexpectedly. This can
// happen if a component reports a fatal error or some other
// async error occurs. Only logs on state changes to avoid
// excessive polling logs.
// See https://github.com/open-telemetry/opentelemetry-collector/blob/8d425480b0dd1270b408582d9e21dd644299cd7e/service/host.go#L34-L39
func (c *baseClient) ensureRunning() {
	c.logger.Info("Ensuring collector is running")
	c.lastKnownState.Store(int32(otelcol.StateStarting))
	for {
		select {
		case <-c.stopped:
			c.logger.Info("Collector is stopped")
			return
		case <-time.After(c.coll.PollInterval):
			currentState := c.coll.GetState()
			lastState := otelcol.State(c.lastKnownState.Load())

			// Only log and react if state has changed
			if currentState != lastState {
				c.lastKnownState.Store(int32(currentState))
				c.logger.Info("Collector state changed", zap.Stringer("previous_state", lastState), zap.Stringer("current_state", currentState))

				if currentState == otelcol.StateClosed && !c.isReloading.Load() {
					c.err <- fmt.Errorf("collector stopped unexpectedly")
				}
			}
		}
	}
}

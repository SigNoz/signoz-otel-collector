package signozcol

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/SigNoz/signoz-otel-collector/components"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/converter/expandconverter"
	"go.opentelemetry.io/collector/confmap/provider/fileprovider"

	"go.opentelemetry.io/collector/otelcol"
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
	wg          sync.WaitGroup
	errChan     chan error
	mux         sync.Mutex
	svc         *otelcol.Collector
}

type WrappedCollectorSettings struct {
	ConfigPaths []string
	Version     string
	Desc        string
	LoggingOpts []zap.Option
}

// New returns a new collector.
func New(settings WrappedCollectorSettings) *WrappedCollector {
	return &WrappedCollector{
		configPaths: settings.ConfigPaths,
		version:     settings.Version,
		desc:        settings.Desc,
		loggingOpts: settings.LoggingOpts,
		errChan:     make(chan error, 1),
	}
}

// Run runs the collector.
func (wCol *WrappedCollector) Run(ctx context.Context) error {
	wCol.mux.Lock()
	defer wCol.mux.Unlock()

	if wCol.svc != nil {
		return fmt.Errorf("collector is already running")
	}

	settings, err := newOtelColSettings(wCol.configPaths, wCol.version, wCol.desc, wCol.loggingOpts)
	if err != nil {
		return err
	}

	// Create a new instance of collector to be used
	svc, err := otelcol.NewCollector(*settings)
	if err != nil {
		return fmt.Errorf("failed to create a new OTel collector service: %w", err)
	}
	wCol.svc = svc

	// Partially copied from
	// https://github.com/open-telemetry/opentelemetry-collector/blob/release/v0.66.x/service/collector_windows.go#L91
	colErrorChannel := make(chan error, 1)

	// https://github.com/open-telemetry/opentelemetry-collector/blob/release/v0.66.x/service/collector.go#L71
	//
	// col.Run blocks until receiving a signal, so needs to be started
	// asynchronously, but it will exit early if an error occurs on startup
	// When we disable graceful shutdown, it doesn't respond to SIGTERM and
	// SIGINT signals, it runs until the shutdown is invoked or some async error
	// occurs.
	wCol.wg.Add(1)
	go func() {
		defer wCol.wg.Done()
		err := svc.Run(ctx)
		// https://github.com/open-telemetry/opentelemetry-collector/blob/release/v0.66.x/service/collector.go#L124
		//
		// The .Shutdown doesn't return an error, it just closes the channel
		// It is then handled by the .Run method
		// If the Shutdown is unsuccessful, the .Run method will return an error
		// and we will return it here

		wCol.reportError(err)
		colErrorChannel <- err
	}()

	// wait until the collector server is in the Running state
	go func() {
		for {
			state := svc.GetState()
			if state == otelcol.StateRunning {
				// TODO: collector may panic or exit unexpectedly, need to handle that
				colErrorChannel <- nil
				break
			}
			time.Sleep(time.Millisecond * 200)

			// Context may be cancelled
			select {
			case <-ctx.Done():
				svc.Shutdown()
				colErrorChannel <- ctx.Err()
				return
			default:
			}
		}
	}()

	// wait until the collector server is in the Running state, or an error was returned
	return <-colErrorChannel
}

// Shutdown stops the collector.
func (wCol *WrappedCollector) Shutdown() {
	wCol.mux.Lock()
	defer wCol.mux.Unlock()

	if wCol.svc != nil {
		wCol.svc.Shutdown()
		wCol.wg.Wait()
		wCol.svc = nil
	}
}

func (wCol *WrappedCollector) reportError(err error) {
	select {
	case wCol.errChan <- err:
	default:
	}
}

// Restart restarts the collector.
func (wCol *WrappedCollector) Restart(ctx context.Context) error {
	wCol.Shutdown()
	return wCol.Run(ctx)
}

func (wCol *WrappedCollector) ErrorChan() <-chan error {
	return wCol.errChan
}

func (wCol *WrappedCollector) GetState() otelcol.State {
	wCol.mux.Lock()
	defer wCol.mux.Unlock()

	if wCol.svc != nil {
		return wCol.svc.GetState()
	}
	return otelcol.StateClosed
}

func newOtelColSettings(configPaths []string, version string, desc string, loggingOpts []zap.Option) (*otelcol.CollectorSettings, error) {
	factories, err := components.Components()
	if err != nil {
		return nil, fmt.Errorf("error while setting up default factories: %w", err)
	}

	buildInfo := component.BuildInfo{
		Command:     os.Args[0],
		Description: desc,
		Version:     version,
	}

	fmp := fileprovider.New()
	configProviderSettings := otelcol.ConfigProviderSettings{
		ResolverSettings: confmap.ResolverSettings{
			URIs:       configPaths,
			Providers:  map[string]confmap.Provider{fmp.Scheme(): fmp},
			Converters: []confmap.Converter{expandconverter.New()},
		},
	}
	provider, err := otelcol.NewConfigProvider(configProviderSettings)
	if err != nil {
		return nil, err
	}

	return &otelcol.CollectorSettings{
		Factories:      factories,
		BuildInfo:      buildInfo,
		LoggingOptions: loggingOpts,
		ConfigProvider: provider,
		// This is set to true to disable the collector to handle SIGTERM and SIGINT on its own.
		DisableGracefulShutdown: true,
	}, nil
}

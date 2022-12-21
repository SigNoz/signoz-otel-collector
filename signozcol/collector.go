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
	"go.opentelemetry.io/collector/service"
	"go.uber.org/zap"
)

type WrappedCollectorSettings struct {
	ConfigPaths []string
	Version     string
	Desc        string
	LoggingOpts []zap.Option
}

type WrappedCollector struct {
	configPaths []string
	version     string
	desc        string
	loggingOpts []zap.Option
	mux         sync.Mutex
	svc         *service.Collector
}

// New returns a new collector.
func New(settings WrappedCollectorSettings) *WrappedCollector {
	return &WrappedCollector{
		configPaths: settings.ConfigPaths,
		version:     settings.Version,
		desc:        settings.Desc,
		loggingOpts: settings.LoggingOpts,
	}
}

// Run runs the collector.
func (c *WrappedCollector) Run(ctx context.Context) error {
	c.mux.Lock()
	defer c.mux.Unlock()

	if c.svc != nil {
		return fmt.Errorf("collector is already running")
	}

	settings, err := newOtelColSettings(c.configPaths, c.version, c.desc, c.loggingOpts)
	if err != nil {
		return err
	}

	// Create a new instance of collector to be used
	svc, err := service.New(*settings)
	if err != nil {
		return fmt.Errorf("failed to create a new OTel collector service: %w", err)
	}
	c.svc = svc

	colErrorChannel := make(chan error, 1)

	// col.Run blocks until receiving a SIGTERM signal, so needs to be started
	// asynchronously, but it will exit early if an error occurs on startup
	go func() {
		colErrorChannel <- svc.Run(ctx)
	}()

	// wait until the collector server is in the Running state
	go func() {
		for {
			state := svc.GetState()
			if state == service.Running {
				// TODO: collector may panic or exit unexpectedly, need to handle that
				colErrorChannel <- nil
				break
			}
			time.Sleep(time.Millisecond * 200)
		}
	}()

	// wait until the collector server is in the Running state, or an error was returned
	return <-colErrorChannel
}

// Shutdown stops the collector.
func (c *WrappedCollector) Shutdown() {
	c.mux.Lock()
	defer c.mux.Unlock()

	if c.svc != nil {
		c.svc.Shutdown()
		c.svc = nil
	}
}

// Restart restarts the collector.
func (c *WrappedCollector) Restart(ctx context.Context) error {
	c.Shutdown()
	return c.Run(ctx)
}

func newOtelColSettings(configPaths []string, version string, desc string, loggingOpts []zap.Option) (*service.CollectorSettings, error) {
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
	configProviderSettings := service.ConfigProviderSettings{
		ResolverSettings: confmap.ResolverSettings{
			URIs:       configPaths,
			Providers:  map[string]confmap.Provider{fmp.Scheme(): fmp},
			Converters: []confmap.Converter{expandconverter.New()},
		},
	}
	provider, err := service.NewConfigProvider(configProviderSettings)
	if err != nil {
		return nil, err
	}

	return &service.CollectorSettings{
		Factories:      factories,
		BuildInfo:      buildInfo,
		LoggingOptions: loggingOpts,
		ConfigProvider: provider,
		// This is set to true to disable the collector to handle SIGTERM and SIGINT on its own.
		DisableGracefulShutdown: true,
	}, nil
}

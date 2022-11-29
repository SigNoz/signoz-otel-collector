package collector

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

// WrappedCollector is an interface for running the OpenTelemetry collector.
type WrappedCollector interface {
	Run(context.Context) error
	Shutdown()
	Restart(context.Context) error
}

type WrappedCollectorSettings struct {
	ConfigPaths []string
	Version     string
	Desc        string
	LoggingOpts []zap.Option
}

type collector struct {
	configPaths []string
	version     string
	desc        string
	loggingOpts []zap.Option
	mux         sync.Mutex
	svc         *service.Collector
}

// New returns a new collector.
func New(settings WrappedCollectorSettings) WrappedCollector {
	return &collector{
		configPaths: settings.ConfigPaths,
		version:     settings.Version,
		desc:        settings.Desc,
		loggingOpts: settings.LoggingOpts,
	}
}

// Run runs the collector.
func (c *collector) Run(ctx context.Context) error {
	c.mux.Lock()
	defer c.mux.Unlock()

	settings, err := newOtelColSettings(c.configPaths, c.version, c.desc, c.loggingOpts)
	if err != nil {
		return err
	}

	// Create a new instance of collector to be used
	svc, err := service.New(*settings)
	if err != nil {
		return fmt.Errorf("failed to create a new OTel collector service: %w", err)
	}

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
func (c *collector) Shutdown() {
	c.mux.Lock()
	defer c.mux.Unlock()

	if c.svc != nil && (c.svc.GetState() == service.Running || c.svc.GetState() == service.Starting) {
		c.svc.Shutdown()
	}
}

// Restart restarts the collector.
func (c *collector) Restart(ctx context.Context) error {
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
		// This is handled by us in the main function runInteractive
		DisableGracefulShutdown: true,
	}, nil
}

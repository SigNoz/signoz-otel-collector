package collectorsimulator

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/SigNoz/signoz-otel-collector/pkg/collectorsimulator/inmemoryexporter"
	"github.com/SigNoz/signoz-otel-collector/pkg/collectorsimulator/inmemoryreceiver"
	"github.com/google/uuid"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/provider/envprovider"
	"go.opentelemetry.io/collector/confmap/provider/fileprovider"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/service"
	"go.opentelemetry.io/collector/service/telemetry/otelconftelemetry"
)

// Puts together a collector service with inmemory receiver and exporter
// for simulating processing of signal data through an otel collector
type CollectorSimulator struct {
	// collector service to be used for the simulation
	collectorSvc *service.Service

	// tmp file where collectorSvc will log errors.
	collectorLogsOutputFilePath string

	// error channel where collector components will report fatal errors
	// Gets passed in as AsyncErrorChannel in service.Settings when creating a collector service.
	collectorErrorChannel chan error

	// Unique ids of inmemory receiver and exporter instances that
	// will be created by collectorSvc
	inMemoryReceiverId string
	inMemoryExporterId string
}

type ConfigGenerator func(baseConfYaml []byte) ([]byte, error)

func NewCollectorSimulator(
	ctx context.Context,
	processorFactories map[component.Type]processor.Factory,
	configGenerator ConfigGenerator,
) (simulator *CollectorSimulator, cleanupFn func(), err error) {
	// Put together collector component factories for use in the simulation
	receiverFactories, err := otelcol.MakeFactoryMap(inmemoryreceiver.NewFactory())
	if err != nil {
		return nil, nil, fmt.Errorf("could not create receiver factories: %w", err)
	}
	exporterFactories, err := otelcol.MakeFactoryMap(inmemoryexporter.NewFactory())
	if err != nil {
		return nil, nil, fmt.Errorf("could not create processor factories: %w", err)
	}
	factories := otelcol.Factories{
		Receivers:  receiverFactories,
		Processors: processorFactories,
		Exporters:  exporterFactories,
		Telemetry:  otelconftelemetry.NewFactory(),
	}

	// Prepare collector config yaml for simulation
	inMemoryReceiverId := uuid.NewString()
	inMemoryExporterId := uuid.NewString()

	logsOutputFile, err := os.CreateTemp("", "collector-simulator-logs-*")
	if err != nil {
		return nil, nil, fmt.Errorf("could not create tmp file for capturing collector logs: %w", err)
	}
	collectorLogsOutputFilePath := logsOutputFile.Name()
	cleanupFn = func() {
		os.Remove(collectorLogsOutputFilePath)
	}
	err = logsOutputFile.Close()
	if err != nil {
		return nil, cleanupFn, fmt.Errorf("could not close tmp collector log file: %w", err)
	}

	collectorConfYaml, err := generateSimulationConfig(
		inMemoryReceiverId,
		configGenerator,
		inMemoryExporterId,
		collectorLogsOutputFilePath,
	)
	if err != nil {
		return nil, cleanupFn, InvalidConfigError(fmt.Errorf("could not generate collector config: %w", err))
	}

	// Read collector config using the same file provider we use in the actual collector.
	// This ensures env variable substitution if any is taken into account.
	simulationConfigFile, err := os.CreateTemp("", "collector-simulator-config-*")
	if err != nil {
		return nil, nil, fmt.Errorf("could not create tmp file for capturing collector logs: %w", err)
	}
	simulationConfigPath := simulationConfigFile.Name()
	cleanupFn = func() {
		os.Remove(collectorLogsOutputFilePath)
		os.Remove(simulationConfigPath)
	}

	_, err = simulationConfigFile.Write(collectorConfYaml)

	if err != nil {
		return nil, cleanupFn, fmt.Errorf("could not write simulation config to tmp file: %w", err)
	}
	err = simulationConfigFile.Close()
	if err != nil {
		return nil, cleanupFn, fmt.Errorf("could not close tmp simulation config file: %w", err)
	}

	fp := fileprovider.NewFactory()
	confProvider, err := otelcol.NewConfigProvider(otelcol.ConfigProviderSettings{
		ResolverSettings: confmap.ResolverSettings{
			URIs:              []string{simulationConfigPath},
			ProviderFactories: []confmap.ProviderFactory{fp, envprovider.NewFactory()},
		},
	})
	if err != nil {
		return nil, cleanupFn, InvalidConfigError(fmt.Errorf("could not create config provider: %w", err))
	}

	collectorCfg, err := confProvider.Get(ctx, factories)
	if err != nil {
		return nil, cleanupFn, InvalidConfigError(fmt.Errorf("failed to parse collector config: %w", err))
	}

	if err = collectorCfg.Validate(); err != nil {
		return nil, cleanupFn, InvalidConfigError(fmt.Errorf("invalid collector config: %w", err))
	}

	// Build and start collector service.
	collectorErrChan := make(chan error)
	svcSettings := service.Settings{
		ReceiversConfigs:   collectorCfg.Receivers,
		ReceiversFactories: factories.Receivers,

		ProcessorsConfigs:   collectorCfg.Processors,
		ProcessorsFactories: factories.Processors,

		ExportersConfigs:   collectorCfg.Exporters,
		ExportersFactories: factories.Exporters,

		ConnectorsConfigs:   collectorCfg.Connectors,
		ConnectorsFactories: factories.Connectors,

		ExtensionsConfigs:   collectorCfg.Extensions,
		ExtensionsFactories: factories.Extensions,

		TelemetryFactory: factories.Telemetry,

		AsyncErrorChannel: collectorErrChan,
	}

	collectorSvc, err := service.New(ctx, svcSettings, collectorCfg.Service)
	if err != nil {
		return nil, cleanupFn, fmt.Errorf("could not instantiate collector service: %w", err)
	}

	return &CollectorSimulator{
		inMemoryReceiverId:          inMemoryReceiverId,
		inMemoryExporterId:          inMemoryExporterId,
		collectorSvc:                collectorSvc,
		collectorErrorChannel:       collectorErrChan,
		collectorLogsOutputFilePath: collectorLogsOutputFilePath,
	}, cleanupFn, nil
}

func (l *CollectorSimulator) Start(ctx context.Context) (
	func(), error,
) {
	// Calling collectorSvc.Start below will in turn call Start on
	// inmemory receiver and exporter instances created by collectorSvc
	//
	// inmemory components are indexed in a global map after Start is called
	// on them and will have to be cleaned up to ensure there is no memory leak
	cleanupFn := func() {
		inmemoryreceiver.CleanupInstance(l.inMemoryReceiverId)
		inmemoryexporter.CleanupInstance(l.inMemoryExporterId)
	}

	err := l.collectorSvc.Start(ctx)
	if err != nil {
		return cleanupFn, fmt.Errorf("could not start collector service for simulation: %w", err)
	}

	return cleanupFn, nil
}

func (l *CollectorSimulator) GetReceiver() *inmemoryreceiver.InMemoryReceiver {
	return inmemoryreceiver.GetReceiverInstance(l.inMemoryReceiverId)
}

func (l *CollectorSimulator) GetExporter() *inmemoryexporter.InMemoryExporter {
	return inmemoryexporter.GetExporterInstance(l.inMemoryExporterId)
}

func (l *CollectorSimulator) Shutdown(ctx context.Context) (
	simulationErrs []string, err error,
) {
	shutdownErr := l.collectorSvc.Shutdown(ctx)

	// Collect all errors logged or reported by collectorSvc
	simulationErrs = []string{}
	close(l.collectorErrorChannel)
	for reportedErr := range l.collectorErrorChannel {
		simulationErrs = append(simulationErrs, reportedErr.Error())
	}

	collectorWarnAndErrorLogs, err := os.ReadFile(l.collectorLogsOutputFilePath)
	if err != nil {
		return nil, fmt.Errorf(
			"could not read collector logs from tmp file: %w", err,
		)
	}
	if len(collectorWarnAndErrorLogs) > 0 {
		errorLines := strings.Split(string(collectorWarnAndErrorLogs), "\n")
		simulationErrs = append(simulationErrs, errorLines...)
	}

	if shutdownErr != nil {
		return simulationErrs, fmt.Errorf(
			"could not shutdown the collector service: %w", shutdownErr,
		)
	}
	return simulationErrs, nil
}

func generateSimulationConfig(
	receiverId string,
	configGenerator ConfigGenerator,
	exporterId string,
	collectorLogsOutputPath string,
) ([]byte, error) {
	baseConf := fmt.Sprintf(`
    receivers:
      memory:
        id: %s
    exporters:
      memory:
        id: %s
    service:
      pipelines:
        logs:
          receivers:
            - memory
          exporters:
            - memory
      telemetry:
        metrics:
          level: none
        logs:
          level: warn
          output_paths: ["%s"]
    `, receiverId, exporterId, collectorLogsOutputPath)

	return configGenerator([]byte(baseConf))
}

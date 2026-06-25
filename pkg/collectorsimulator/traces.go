package collectorsimulator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
)

// Simulate processing of traces through the otel collector.
// Useful for testing, validation and generating previews.
func SimulateTracesProcessing(ctx context.Context, processorFactories map[component.Type]processor.Factory, configGenerator ConfigGenerator, traces []ptrace.Traces, timeout time.Duration) (outputTraces []ptrace.Traces, collectorErrs []string, err error) {
	simulator, simulatorInitCleanup, err := NewCollectorSimulator(
		ctx, SignalTraces, processorFactories, configGenerator,
	)
	if simulatorInitCleanup != nil {
		defer simulatorInitCleanup()
	}
	if err != nil {
		return nil, nil, fmt.Errorf("could not create traces processing simulator: %w", err)
	}

	simulatorCleanup, err := simulator.Start(ctx)
	if simulatorCleanup != nil {
		defer simulatorCleanup()
	}
	if err != nil {
		return nil, nil, err
	}

	for _, td := range traces {
		err = SendTracesToSimulator(ctx, simulator, td)
		if err != nil {
			return nil, nil, fmt.Errorf("could not consume traces for simulation: %w", err)
		}
	}

	result, err := GetProcessedTracesFromSimulator(
		simulator, len(traces), timeout,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("could not get processed traces from simulator: %w", err)
	}

	simulationErrs, err := simulator.Shutdown(ctx)
	if err != nil {
		return nil, simulationErrs, fmt.Errorf("could not shutdown traces processing simulator: %w", err)
	}

	for _, log := range simulationErrs {
		if log == "" || strings.Contains(log, "featuregate.go") {
			continue
		}
		collectorErrs = append(collectorErrs, log)
	}

	return result, collectorErrs, nil
}

func SendTracesToSimulator(ctx context.Context, simulator *CollectorSimulator, td ptrace.Traces) error {
	receiver := simulator.GetReceiver()
	if receiver == nil {
		return fmt.Errorf("could not find in memory receiver for simulator")
	}
	if err := receiver.ConsumeTraces(ctx, td); err != nil {
		return fmt.Errorf("inmemory receiver could not consume traces for simulation: %w", err)
	}
	return nil
}

func GetProcessedTracesFromSimulator(simulator *CollectorSimulator, minTraceCount int, timeout time.Duration) ([]ptrace.Traces, error) {
	exporter := simulator.GetExporter()
	if exporter == nil {
		return nil, fmt.Errorf("could not find in memory exporter for simulator")
	}

	startTsMillis := time.Now().UnixMilli()
	for {
		elapsedMillis := time.Now().UnixMilli() - startTsMillis
		if elapsedMillis > timeout.Milliseconds() {
			break
		}

		exportedTraces := exporter.GetTraces()
		if len(exportedTraces) >= minTraceCount {
			return exportedTraces, nil
		}

		time.Sleep(50 * time.Millisecond)
	}

	return exporter.GetTraces(), nil
}

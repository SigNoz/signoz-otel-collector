// An async stanza-based implementation of the OTel collector processor.Logs.
// Architecture mirrors opentelemetry-collector-contrib's logstransformprocessor:
// a FromPdataConverter worker pool converts incoming plog.Logs into stanza
// entries; a single converterLoop goroutine drains the converter and feeds the
// stanza pipeline serially; a BatchingLogEmitter at the pipeline's tail
// forwards processed entries to the next consumer.
//
// Trade-off: ConsumeLogs returns immediately after enqueuing the batch.
// Backpressure to the OTel batchprocessor and error propagation from the next
// consumer are NOT preserved.
package signozlogspipelineprocessor

import (
	"context"
	"errors"
	"math"
	"runtime"
	"sync"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/pipeline"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator/operators/router"

	_ "github.com/SigNoz/signoz-otel-collector/pkg/parser/grok" // ensure grok parser gets registered.
)

type logsPipelineProcessor struct {
	telemetrySettings component.TelemetrySettings
	processorConfig   *Config

	consumer consumer.Logs

	pipe          *pipeline.DirectedPipeline
	firstOperator operator.Operator
	emitter       *helper.BatchingLogEmitter
	fromConverter *adapter.FromPdataConverter
	shutdownFns   []component.ShutdownFunc
}

func newLogsPipelineProcessor(
	processorConfig *Config,
	telemetrySettings component.TelemetrySettings,
	nextConsumer consumer.Logs,
) (*logsPipelineProcessor, error) {
	p := &logsPipelineProcessor{
		telemetrySettings: telemetrySettings,
		processorConfig:   processorConfig,
		consumer:          nextConsumer,
	}

	p.emitter = helper.NewBatchingLogEmitter(telemetrySettings, p.consumeStanzaLogEntries)

	pipe, err := pipeline.Config{
		Operators:     processorConfig.OperatorConfigs(),
		DefaultOutput: p.emitter,
	}.Build(telemetrySettings)
	if err != nil {
		return nil, err
	}
	p.pipe = pipe

	return p, nil
}

func (*logsPipelineProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (p *logsPipelineProcessor) Start(ctx context.Context, _ component.Host) error {
	p.telemetrySettings.Logger.Info("Starting logsPipelineProcessor")

	wkrCount := int(math.Max(1, float64(runtime.NumCPU())))
	p.fromConverter = adapter.NewFromPdataConverter(p.telemetrySettings, wkrCount)

	// Start in reverse order of the data flow (pipeline → converterLoop → fromConverter)
	// so that the pipeline is ready before entries start arriving.
	if err := p.startPipeline(); err != nil {
		return err
	}
	p.startConverterLoop(ctx)
	p.startFromConverter()

	return nil
}

func (p *logsPipelineProcessor) Shutdown(ctx context.Context) error {
	p.telemetrySettings.Logger.Info("Stopping logsPipelineProcessor")

	// Call shutdown functions in reverse order of registration: fromConverter
	// stops first (no new entries), then converterLoop drains, then the
	// pipeline stops.
	for i := len(p.shutdownFns) - 1; i >= 0; i-- {
		fn := p.shutdownFns[i]
		if err := fn(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (p *logsPipelineProcessor) startPipeline() error {
	// This processor does not use stanza's persistent storage.
	if err := p.pipe.Start(storage.NewNopClient()); err != nil {
		return err
	}
	p.shutdownFns = append(p.shutdownFns, func(_ context.Context) error {
		return p.pipe.Stop()
	})

	ops := p.pipe.Operators()
	if len(ops) == 0 {
		return errors.New("processor requires at least one operator to be configured")
	}
	p.firstOperator = ops[0]

	// Preserve the pre-async behavior where entries that no router route
	// matched still flowed through to the next consumer. Wire every router's
	// fallthrough output to the emitter so unmatched entries are forwarded
	// directly (skipping any downstream operators, which is what the old
	// sync implementation effectively did because it returned the full input
	// entry slice regardless of pipeline forwarding).
	if r, ok := p.firstOperator.(*router.Transformer); ok {
		r.SetDefaultOutput(p.emitter)
	}

	return nil
}

func (p *logsPipelineProcessor) startConverterLoop(ctx context.Context) {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go p.converterLoop(ctx, wg)
	p.shutdownFns = append(p.shutdownFns, func(_ context.Context) error {
		wg.Wait()
		return nil
	})
}

func (p *logsPipelineProcessor) startFromConverter() {
	p.fromConverter.Start()
	p.shutdownFns = append(p.shutdownFns, func(_ context.Context) error {
		p.fromConverter.Stop()
		return nil
	})
}

func (p *logsPipelineProcessor) ConsumeLogs(_ context.Context, ld plog.Logs) error {
	return p.fromConverter.Batch(ld)
}

// converterLoop reads stanza entries produced by fromConverter and feeds them
// serially into the first operator of the stanza pipeline.
func (p *logsPipelineProcessor) converterLoop(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			p.telemetrySettings.Logger.Debug("converter loop stopped")
			return

		case entries, ok := <-p.fromConverter.OutChannel():
			if !ok {
				p.telemetrySettings.Logger.Debug("fromConverter channel got closed")
				return
			}

			for _, e := range entries {
				if err := p.firstOperator.Process(ctx, e); err != nil {
					p.telemetrySettings.Logger.Error("processor encountered an issue with the pipeline", zap.Error(err))
					break
				}
			}
		}
	}
}

// consumeStanzaLogEntries is the BatchingLogEmitter callback: a batch of
// processed entries arrives here, gets converted back to plog, and is
// forwarded to the next consumer. Errors are logged but not propagated.
//
// We deliberately use the local convertEntriesToPlogs (utils.go) instead of
// adapter.ConvertEntries because the local version filters out attributes
// prefixed with signozstanzaentry.InternalTempAttributePrefix, which the
// SigNoz operator suite uses for scratch state. Those keys must not be
// emitted to the next consumer.
func (p *logsPipelineProcessor) consumeStanzaLogEntries(ctx context.Context, entries []*entry.Entry) {
	pLogs := convertEntriesToPlogs(entries)
	if err := p.consumer.ConsumeLogs(ctx, pLogs); err != nil {
		p.telemetrySettings.Logger.Error("processor encountered an issue with next consumer", zap.Error(err))
	}
}

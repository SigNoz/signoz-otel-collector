// A sync implementation of stanza based otel collector processor.Logs
// logstransform processor in opentelemetry-collector-contrib is async
package signozlogspipelineprocessor

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"runtime"
	"sync"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/pipeline"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	_ "github.com/SigNoz/signoz-otel-collector/pkg/parser/grok" // ensure grok parser gets registered.
	"github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/internal/metadata"
)

func newLogsPipelineProcessor(
	processorConfig *Config,
	nextConsumer consumer.Logs,
	telemetrySettings component.TelemetrySettings,
) (*logsPipelineProcessor, error) {

	telemetrySettings.Logger.Info("number of CPUs", zap.Int("num", runtime.NumCPU()))
	meter := telemetrySettings.MeterProvider.Meter(metadata.ScopeName)
	durationHistogram, err := meter.Float64Histogram(
		"pipelines_processing_latency",
		metric.WithDescription("Time taken for entries to process"),
	)
	if err != nil {
		return nil, err
	}

	p := &logsPipelineProcessor{
		telemetrySettings: telemetrySettings,
		durationHistogram: durationHistogram,
		consumer:          nextConsumer,
		// batchSize:       10_000,
		// limiter:         make(chan struct{}, runtime.NumCPU()),
		processorConfig: processorConfig,
	}

	p.emitter = helper.NewBatchingLogEmitter(p.telemetrySettings, p.consumeStanzaLogEntries)
	p.stanzaPipeline, err = pipeline.Config{
		Operators:     processorConfig.OperatorConfigs(),
		DefaultOutput: p.emitter,
	}.Build(telemetrySettings)
	if err != nil {
		return nil, err
	}

	return p, nil
}

type logsPipelineProcessor struct {
	telemetrySettings component.TelemetrySettings
	durationHistogram metric.Float64Histogram

	processorConfig *Config
	consumer        consumer.Logs
	stanzaPipeline  *pipeline.DirectedPipeline
	firstOp         operator.Operator
	// limiter         chan struct{}
	// batchSize       int
	emitter       helper.LogEmitter
	fromConverter *adapter.FromPdataConverter
	shutdownFns   []component.ShutdownFunc
}

// Collector starting up
func (p *logsPipelineProcessor) Start(ctx context.Context, _ component.Host) error {
	p.telemetrySettings.Logger.Info("Starting logsPipelineProcessor")
	wkrCount := int(math.Max(1, float64(runtime.NumCPU())))
	p.fromConverter = adapter.NewFromPdataConverter(p.telemetrySettings, wkrCount)

	// data flows in this order:
	// ConsumeLogs: receives logs and forwards them for conversion to stanza format ->
	// fromConverter: converts logs to stanza format ->
	// converterLoop: forwards converted logs to the stanza pipeline ->
	// pipeline: performs user configured operations on the logs ->
	// transformProcessor: receives []*entry.Entries, converts them to plog.Logs and sends the converted OTLP logs to the next consumer
	//
	// We should start these components in reverse order of the data flow, then stop them in order of the data flow,
	// in order to allow for pipeline draining.
	err := p.startPipeline()
	if err != nil {
		return err
	}

	p.startConverterLoop(ctx)
	p.startFromConverter()

	return nil
}

func (p *logsPipelineProcessor) startFromConverter() {
	p.fromConverter.Start()

	p.shutdownFns = append(p.shutdownFns, func(_ context.Context) error {
		p.fromConverter.Stop()
		return nil
	})
}

// startConverterLoop starts the converter loop, which reads all the logs translated by the fromConverter and then forwards
// them to pipeline
func (p *logsPipelineProcessor) startConverterLoop(ctx context.Context) {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go p.converterLoop(ctx, wg)

	p.shutdownFns = append(p.shutdownFns, func(_ context.Context) error {
		wg.Wait()
		return nil
	})
}

func (p *logsPipelineProcessor) startPipeline() error {
	// There is no need for this processor to use storage
	err := p.stanzaPipeline.Start(storage.NewNopClient())
	if err != nil {
		return err
	}

	p.shutdownFns = append(p.shutdownFns, func(_ context.Context) error {
		return p.stanzaPipeline.Stop()
	})

	pipelineOperators := p.stanzaPipeline.Operators()
	if len(pipelineOperators) == 0 {
		return errors.New("processor requires at least one operator to be configured")
	}
	p.firstOp = pipelineOperators[0]

	return nil
}

// Collector shutting down
func (p *logsPipelineProcessor) Shutdown(ctx context.Context) error {
	p.telemetrySettings.Logger.Info("Stopping logsPipelineProcessor")

	// Call shutdown functions in reverse order, so that shutdown happens in LIFO order of starting
	for i := len(p.shutdownFns) - 1; i >= 0; i-- {
		fn := p.shutdownFns[i]

		if err := fn(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (p *logsPipelineProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

// func (p *logsPipelineProcessor) ProcessLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
// 	p.telemetrySettings.Logger.Debug(
// 		"logsPipelineProcessor received logs",
// 		zap.Any("logs", ld),
// 	)

// 	entries := plogToEntries(ld)

// 	start := time.Now()

// 	process := func(ctx context.Context, entries []*entry.Entry) {
// 		for _, entry := range entries {
// 			if err := p.firstOp.Process(ctx, entry); err != nil {
// 				p.telemetrySettings.Logger.Error("processor encountered an issue with the pipeline", zap.Error(err))
// 			}
// 		}
// 	}

// 	group, groupCtx := errgroup.WithContext(ctx)
// 	// group.SetLimit(runtime.NumCPU() * 5)
// 	for _, batch := range utils.Batch(entries, p.batchSize) {
// 		select {
// 		case p.limiter <- struct{}{}:
// 			group.Go(func() error {
// 				defer func() {
// 					<-p.limiter
// 				}()
// 				process(groupCtx, batch)
// 				return nil // not returning error to avoid cancelling groupCtx
// 			})
// 		default:
// 			process(ctx, batch)
// 		}
// 	}

// 	// wait for the group execution
// 	_ = group.Wait()

// 	p.durationHistogram.Record(ctx,
// 		float64(time.Since(start).Seconds()),
// 		metric.WithAttributes(
// 			attribute.String("step", "firstOpProcess"),
// 			attribute.Int("total_entries_processed", len(entries)),
// 		),
// 	)

// 	// All stanza ops supported by logs pipelines work synchronously and
// 	// they modify the *entry.Entry passed to them in-place.
// 	//
// 	// So by this point, `entries` contains processed logs
// 	plog := convertEntriesToPlogs(entries)
// 	return plog, nil
// }

func (p *logsPipelineProcessor) ConsumeLogs(_ context.Context, ld plog.Logs) error {
	// Add the logs to the chain
	return p.fromConverter.Batch(ld)
}

// converterLoop reads the log entries produced by the fromConverter and sends them
// into the pipeline
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
				// Add item to the first operator of the pipeline manually
				if err := p.firstOp.Process(ctx, e); err != nil {
					p.telemetrySettings.Logger.Error("processor encountered an issue with the pipeline", zap.Error(err))
					break
				}
			}
		}
	}
}

func (p *logsPipelineProcessor) consumeStanzaLogEntries(ctx context.Context, entries []*entry.Entry) {
	pLogs := adapter.ConvertEntries(entries)
	if err := p.consumer.ConsumeLogs(ctx, pLogs); err != nil {
		p.telemetrySettings.Logger.Error("processor encountered an issue with next consumer", zap.Error(err))
	}
}

// Helpers below have been brought in as is from stanza adapter for logstransform
func plogToEntries(src plog.Logs) []*entry.Entry {
	result := []*entry.Entry{}
	for rlIdx := 0; rlIdx < src.ResourceLogs().Len(); rlIdx++ {
		resourceLogs := src.ResourceLogs().At(rlIdx)

		for slIdx := 0; slIdx < resourceLogs.ScopeLogs().Len(); slIdx++ {
			scopeLogs := resourceLogs.ScopeLogs().At(slIdx)

			for lrIdx := 0; lrIdx < scopeLogs.LogRecords().Len(); lrIdx++ {
				record := scopeLogs.LogRecords().At(lrIdx)
				entry := entry.Entry{}
				entry.ScopeName = scopeLogs.Scope().Name()
				// each entry has separate reference for Resource Tags since these're used in processor, Resource tags can be transformed based on user's pipelines
				entry.Resource = resourceLogs.Resource().Attributes().AsRaw()
				convertFrom(record, &entry)
				result = append(result, &entry)
			}
		}
	}

	return result
}

func convertFrom(src plog.LogRecord, ent *entry.Entry) {
	// if src.Timestamp == 0, then leave ent.Timestamp as nil
	if src.Timestamp() != 0 {
		ent.Timestamp = src.Timestamp().AsTime()
	}

	if src.ObservedTimestamp() == 0 {
		ent.ObservedTimestamp = time.Now()
	} else {
		ent.ObservedTimestamp = src.ObservedTimestamp().AsTime()
	}

	ent.Severity = fromPdataSevMap[src.SeverityNumber()]
	ent.SeverityText = src.SeverityText()

	ent.Attributes = src.Attributes().AsRaw()
	ent.Body = src.Body().AsRaw()

	if !src.TraceID().IsEmpty() {
		buffer := src.TraceID()
		ent.TraceID = buffer[:]
	}
	if !src.SpanID().IsEmpty() {
		buffer := src.SpanID()
		ent.SpanID = buffer[:]
	}
	if src.Flags() != 0 {
		a := make([]byte, 4)
		binary.LittleEndian.PutUint32(a, uint32(src.Flags()))
		ent.TraceFlags = []byte{a[0]}
	}
}

var fromPdataSevMap = map[plog.SeverityNumber]entry.Severity{
	plog.SeverityNumberUnspecified: entry.Default,
	plog.SeverityNumberTrace:       entry.Trace,
	plog.SeverityNumberTrace2:      entry.Trace2,
	plog.SeverityNumberTrace3:      entry.Trace3,
	plog.SeverityNumberTrace4:      entry.Trace4,
	plog.SeverityNumberDebug:       entry.Debug,
	plog.SeverityNumberDebug2:      entry.Debug2,
	plog.SeverityNumberDebug3:      entry.Debug3,
	plog.SeverityNumberDebug4:      entry.Debug4,
	plog.SeverityNumberInfo:        entry.Info,
	plog.SeverityNumberInfo2:       entry.Info2,
	plog.SeverityNumberInfo3:       entry.Info3,
	plog.SeverityNumberInfo4:       entry.Info4,
	plog.SeverityNumberWarn:        entry.Warn,
	plog.SeverityNumberWarn2:       entry.Warn2,
	plog.SeverityNumberWarn3:       entry.Warn3,
	plog.SeverityNumberWarn4:       entry.Warn4,
	plog.SeverityNumberError:       entry.Error,
	plog.SeverityNumberError2:      entry.Error2,
	plog.SeverityNumberError3:      entry.Error3,
	plog.SeverityNumberError4:      entry.Error4,
	plog.SeverityNumberFatal:       entry.Fatal,
	plog.SeverityNumberFatal2:      entry.Fatal2,
	plog.SeverityNumberFatal3:      entry.Fatal3,
	plog.SeverityNumberFatal4:      entry.Fatal4,
}

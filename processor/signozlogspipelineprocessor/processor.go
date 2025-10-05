// A sync implementation of stanza based otel collector processor.Logs
// logstransform processor in opentelemetry-collector-contrib is async
package signozlogspipelineprocessor

import (
	"context"
	"encoding/binary"
	"math"
	"runtime"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/pipeline"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	_ "github.com/SigNoz/signoz-otel-collector/pkg/parser/grok" // ensure grok parser gets registered.
)

func newLogsPipelineProcessor(
	processorConfig *Config,
	telemetrySettings component.TelemetrySettings,
) (*logsPipelineProcessor, error) {

	stanzaPipeline, err := pipeline.Config{
		Operators: processorConfig.OperatorConfigs(),
	}.Build(telemetrySettings)
	if err != nil {
		return nil, err
	}

	concurrency := int(math.Max(1, float64(runtime.GOMAXPROCS(0))))

	telemetrySettings.Logger.Info("logspipeline concurrency set to ", zap.Int("num", concurrency))

	return &logsPipelineProcessor{
		telemetrySettings: telemetrySettings,

		limiter:         make(chan struct{}, concurrency),
		processorConfig: processorConfig,
		stanzaPipeline:  stanzaPipeline,
	}, nil
}

type logsPipelineProcessor struct {
	telemetrySettings component.TelemetrySettings

	processorConfig *Config
	stanzaPipeline  *pipeline.DirectedPipeline
	firstOp         operator.Operator
	limiter         chan struct{}
	shutdownFns     []component.ShutdownFunc
}

// Collector starting up
func (p *logsPipelineProcessor) Start(ctx context.Context, _ component.Host) error {
	p.telemetrySettings.Logger.Info("Starting logsPipelineProcessor")

	err := p.stanzaPipeline.Start(storage.NewNopClient())
	if err != nil {
		return err
	}

	p.shutdownFns = append(p.shutdownFns, func(_ context.Context) error {
		return p.stanzaPipeline.Stop()
	})

	// .Operators() returns topologically sorted operators for the stanza operator graph
	// logs will be fed into p.firstOp for processing
	p.firstOp = p.stanzaPipeline.Operators()[0]

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

func (p *logsPipelineProcessor) ProcessLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	p.telemetrySettings.Logger.Debug(
		"logsPipelineProcessor received logs",
		zap.Any("logs", ld),
	)

	entries := plogToEntries(ld)

	process := func(ctx context.Context, entry *entry.Entry) {
		if err := p.firstOp.Process(ctx, entry); err != nil {
			p.telemetrySettings.Logger.Error("processor encountered an issue with the pipeline", zap.Error(err))
		}
	}

	group, groupCtx := errgroup.WithContext(ctx)
	for _, entry := range entries {
		p.limiter <- struct{}{}
		group.Go(func() error {
			defer func() {
				<-p.limiter
			}()
			process(groupCtx, entry)
			return nil // not returning error to avoid cancelling groupCtx
		})
	}

	// wait for the group execution
	_ = group.Wait()

	// All stanza ops supported by logs pipelines work synchronously and
	// they modify the *entry.Entry passed to them in-place.
	//
	// So by this point, `entries` contains processed logs
	plog := convertEntriesToPlogs(entries)
	return plog, nil
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

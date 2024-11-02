package signozmemorylimiterprocessor

import (
	"context"
	"errors"
	"runtime"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"
)

const (
	mibBytes uint64 = 1024 * 1024
)

var (
	ErrDataRefused = errors.New("data refused due to high memory usage")
)

type memoryLimiterProcessor struct {
	cfg            *Config
	set            processor.Settings
	readMemStatsFn func(m *runtime.MemStats)
	usageChecker   *memUsageChecker
}

func newMemoryLimiterProcessor(set processor.Settings, cfg *Config) (*memoryLimiterProcessor, error) {
	usageChecker := newFixedMemUsageChecker(uint64(cfg.MemoryLimitMiB) * mibBytes)

	return &memoryLimiterProcessor{
		cfg:            cfg,
		set:            set,
		readMemStatsFn: runtime.ReadMemStats,
		usageChecker:   usageChecker,
	}, nil
}

func (processor *memoryLimiterProcessor) Start(ctx context.Context, host component.Host) error {
	return nil
}

func (processor *memoryLimiterProcessor) Shutdown(ctx context.Context) error {
	return nil
}

func (processor *memoryLimiterProcessor) ConsumeTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	return td, processor.checkMemory()
}

func (processor *memoryLimiterProcessor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	return md, processor.checkMemory()
}

func (processor *memoryLimiterProcessor) ConsumeLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	return ld, processor.checkMemory()
}

func (processor *memoryLimiterProcessor) checkMemory() error {
	ms := processor.readMemStats()

	if processor.usageChecker.aboveLimit(ms) {
		processor.set.Logger.Error("Currently used memory is higher then limit memory.", memstatToZapField(ms), zap.Uint32("limit_mib", processor.cfg.MemoryLimitMiB))
		return ErrDataRefused
	}

	return nil
}

func (processor *memoryLimiterProcessor) readMemStats() *runtime.MemStats {
	ms := &runtime.MemStats{}
	processor.readMemStatsFn(ms)
	return ms
}

func memstatToZapField(ms *runtime.MemStats) zap.Field {
	return zap.Uint64("cur_mem_mib", ms.Alloc/mibBytes)
}

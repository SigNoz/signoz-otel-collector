package signozfilterprocessor

import (
	"context"
	"time"

	"github.com/SigNoz/signoz-otel-collector/pkg/filterrule"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

type filterProcessor struct {
	maxAge time.Duration
}

func newProcessor(cfg *Config) *filterProcessor {
	return &filterProcessor{maxAge: time.Duration(cfg.MaxAllowedDataAgeDays) * 24 * time.Hour}
}

func (p *filterProcessor) ProcessLogs(_ context.Context, ld plog.Logs) (plog.Logs, error) {
	oldestAllowed := filterrule.OldestAllowed(time.Now(), p.maxAge)
	ld.ResourceLogs().RemoveIf(func(rl plog.ResourceLogs) bool {
		rl.ScopeLogs().RemoveIf(func(sl plog.ScopeLogs) bool {
			sl.LogRecords().RemoveIf(func(record plog.LogRecord) bool {
				return filterrule.LogRecordTooOld(record, oldestAllowed)
			})
			return sl.LogRecords().Len() == 0
		})
		return rl.ScopeLogs().Len() == 0
	})
	if ld.ResourceLogs().Len() == 0 {
		return ld, processorhelper.ErrSkipProcessingData
	}
	return ld, nil
}

func (p *filterProcessor) ProcessTraces(_ context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	oldestAllowed := filterrule.OldestAllowed(time.Now(), p.maxAge)
	td.ResourceSpans().RemoveIf(func(rs ptrace.ResourceSpans) bool {
		rs.ScopeSpans().RemoveIf(func(ss ptrace.ScopeSpans) bool {
			ss.Spans().RemoveIf(func(span ptrace.Span) bool {
				return filterrule.SpanTooOld(span, oldestAllowed)
			})
			return ss.Spans().Len() == 0
		})
		return rs.ScopeSpans().Len() == 0
	})
	if td.ResourceSpans().Len() == 0 {
		return td, processorhelper.ErrSkipProcessingData
	}
	return td, nil
}

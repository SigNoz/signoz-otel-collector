package sampling

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
)

type alwaysUnSample struct {
	logger *zap.Logger
}

var _ PolicyEvaluator = (*alwaysUnSample)(nil)

// NewAlwaysSample creates a policy evaluator the samples all traces.
func NewAlwaysUnsample(logger *zap.Logger) PolicyEvaluator {
	return &alwaysUnSample{
		logger: logger,
	}
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
func (aus *alwaysUnSample) Evaluate(pcommon.TraceID, *TraceData) (Decision, error) {
	aus.logger.Debug("Evaluating spans in always-sample filter")
	return NotSampled, nil
}

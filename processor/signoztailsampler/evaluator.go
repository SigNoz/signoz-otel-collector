package signoztailsamplingprocessor

import (
	"github.com/SigNoz/signoz-otel-collector/processor/signoztailsampler/internal/sampling"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
)

// default evaluator for policyCfg.
type defaultEvaluator struct {
	name string

	// true for top level policies
	root bool

	priority int

	// in case probalistic sampling to be done when filter matches
	sampler sampling.PolicyEvaluator

	// can be probabilistic, always_include, always_exclude
	samplingMethod string

	filterOperator string
	filters        []sampling.PolicyEvaluator

	// the subpolicy evaluators will be processed in order of priority.
	// the sub-policies will be evaluated prior to evaluating top-level policy,
	// if any of subpolicy filters match, the sampling method associated with that
	// sub-policy will be applied. and no further processing will be performed.
	subpolicies []sampling.PolicyEvaluator
	logger      *zap.Logger
}

func NewDefaultEvaluator(logger *zap.Logger, policyCfg *PolicyCfg) sampling.PolicyEvaluator {

	// condition operator to apply on filters (AND | OR)
	filterOperator := policyCfg.PolicyFilterCfg.FilterOp

	// list of filter evaluators used to decide if this policy
	// should be applied
	filters := prepareFilterEvaluators(logger, policyCfg.PolicyFilterCfg)
	// todo(amol): need to handle situations with zero filters

	// list of sub-policies evaluators
	subpolicies := make([]sampling.PolicyEvaluator, len(policyCfg.SubPolicies))
	for _, subRule := range policyCfg.SubPolicies {
		subPolicy := NewDefaultEvaluator(logger, &subRule)
		subpolicies = append(subpolicies, subPolicy)
	}

	// sampling is applied only when filter conditions are met
	var sampler sampling.PolicyEvaluator
	var samplingMethod string
	switch policyCfg.SamplingPercentage {
	case 0:
		samplingMethod = "exclude_all"
		sampler = sampling.NewAlwaysUnsample(logger)
	case 100:
		samplingMethod = "include_all"
		sampler = sampling.NewAlwaysSample(logger)
	default:
		samplingMethod = "probabilistic"
		sampler = sampling.NewProbabilisticSampler(logger, "21d2", policyCfg.SamplingPercentage)
	}

	return &defaultEvaluator{
		name:           policyCfg.Name,
		root:           policyCfg.Root,
		sampler:        sampler,
		samplingMethod: samplingMethod,
		filterOperator: filterOperator,
		filters:        filters,
		subpolicies:    subpolicies,
	}
}

func prepareFilterEvaluators(logger *zap.Logger, policyFilterCfg PolicyFilterCfg) []sampling.PolicyEvaluator {
	var filterEvaluators []sampling.PolicyEvaluator
	for _, s := range policyFilterCfg.StringAttributeCfgs {
		filterEvaluators = append(filterEvaluators, sampling.NewStringAttributeFilter(logger, s.Key, s.Values, s.EnabledRegexMatching, s.CacheMaxSize, s.InvertMatch))
	}

	for _, n := range policyFilterCfg.NumericAttributeCfgs {
		filterEvaluators = append(filterEvaluators, sampling.NewNumericAttributeFilter(logger, n.Key, n.MinValue, n.MaxValue))
	}
	// todo: status filter
	return filterEvaluators
}

func (de *defaultEvaluator) Evaluate(traceId pcommon.TraceID, trace *sampling.TraceData) (sampling.Decision, error) {

	// todo: explore doing this in parallel
	for _, sp := range de.subpolicies {
		decision, err := sp.Evaluate(traceId, trace)
		if err != nil {
			// todo: consider adding health for each evaluator
			// to avoid printing log messages for each trace
			zap.S().Errorf("failed to evaluate trace:", traceId)
			continue
		}

		if decision != sampling.NoResult {
			// found a result, exit
			return decision, nil
		}
	}

	// filterMatched captures when atleast one filter matches
	var filterMatched bool

	for _, fe := range de.filters {

		filterResult, err := fe.Evaluate(traceId, trace)
		if err != nil {
			return sampling.Error, nil
		}
		// since filters can also be sampling evaluators,
		// here we consider "sampled = filter matched" and
		// "no sampled == filter did not match"

		if filterResult == sampling.Sampled {
			filterMatched = true
		}

		if filterResult == sampling.NotSampled {
			if de.filterOperator == "AND" {
				// filter condition failed, return no op
				return sampling.NoResult, nil
			}
		}
	}

	if filterMatched {
		// filter conditions have matched, let us
		// apply sampling action now
		return de.sampler.Evaluate(traceId, trace)
	}

	return sampling.NoResult, nil
}

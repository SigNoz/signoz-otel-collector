// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package signoztailsampler

import (
	"time"

	"go.opentelemetry.io/collector/config"
)

// PolicyType indicates the type of sampling policy.
type PolicyType string

const (
	// AlwaysSample samples all traces, typically used for debugging.
	AlwaysSample PolicyType = "always_sample"
	// Latency sample traces that are longer than a given threshold.
	Latency PolicyType = "latency"
	// NumericAttribute sample traces that have a given numeric attribute in a specified
	// range, e.g.: attribute "http.status_code" >= 399 and <= 999.
	NumericAttribute PolicyType = "numeric_attribute"
	// Probabilistic samples a given percentage of traces.
	Probabilistic PolicyType = "probabilistic"
	// StatusCode sample traces that have a given status code.
	StatusCode PolicyType = "status_code"
	// StringAttribute sample traces that a attribute, of type string, matching
	// one of the listed values.
	StringAttribute PolicyType = "string_attribute"
	// RateLimiting allows all traces until the specified limits are satisfied.
	RateLimiting PolicyType = "rate_limiting"
	// Composite allows defining a composite policy, combining the other policies in one
	Composite PolicyType = "composite"
	// And allows defining a And policy, combining the other policies in one
	And PolicyType = "and"
	// SpanCount sample traces that are have more spans per Trace than a given threshold.
	SpanCount PolicyType = "span_count"
	// TraceState sample traces with specified values by the given key
	TraceState PolicyType = "trace_state"

	// PolicyGroup allows grouping of rules and priority based sampling
	PolicyGroup = "policy_group"
)

type TraceStateCfg struct {
	// Tag that the filter is going to be matching against.
	Key string `mapstructure:"key"`
	// Values indicate the set of values to use when matching against trace_state values.
	Values []string `mapstructure:"values"`
}

// RateAllocationCfg  used within composite policy
type RateAllocationCfg struct {
	Policy  string `mapstructure:"policy"`
	Percent int64  `mapstructure:"percent"`
}

// LatencyCfg holds the configurable settings to create a latency filter sampling policy
// evaluator
type LatencyCfg struct {
	// ThresholdMs in milliseconds.
	ThresholdMs int64 `mapstructure:"threshold_ms"`
}

// NumericAttributeCfg holds the configurable settings to create a numeric attribute filter
// sampling policy evaluator.
type NumericAttributeCfg struct {
	// Tag that the filter is going to be matching against.
	Key string `mapstructure:"key"`
	// MinValue is the minimum value of the attribute to be considered a match.
	MinValue int64 `mapstructure:"min_value"`
	// MaxValue is the maximum value of the attribute to be considered a match.
	MaxValue int64 `mapstructure:"max_value"`
}

// ProbabilisticCfg holds the configurable settings to create a probabilistic
// sampling policy evaluator.
type ProbabilisticCfg struct {
	// HashSalt allows one to configure the hashing salts. This is important in scenarios where multiple layers of collectors
	// have different sampling rates: if they use the same salt all passing one layer may pass the other even if they have
	// different sampling rates, configuring different salts avoids that.
	HashSalt string `mapstructure:"hash_salt"`
	// SamplingPercentage is the percentage rate at which traces are going to be sampled. Defaults to zero, i.e.: no sample.
	// Values greater or equal 100 are treated as "sample all traces".
	SamplingPercentage float64 `mapstructure:"sampling_percentage"`
}

// StatusCodeCfg holds the configurable settings to create a status code filter sampling
// policy evaluator.
type StatusCodeCfg struct {
	StatusCodes []string `mapstructure:"status_codes"`
}

// StringAttributeCfg holds the configurable settings to create a string attribute filter
// sampling policy evaluator.
type StringAttributeCfg struct {
	// Tag that the filter is going to be matching against.
	Key string `mapstructure:"key"`
	// Values indicate the set of values or regular expressions to use when matching against attribute values.
	// StringAttribute Policy will apply exact value match on Values unless EnabledRegexMatching is true.
	Values []string `mapstructure:"values"`
	// EnabledRegexMatching determines whether match attribute values by regexp string.
	EnabledRegexMatching bool `mapstructure:"enabled_regex_matching"`
	// CacheMaxSize is the maximum number of attribute entries of LRU Cache that stores the matched result
	// from the regular expressions defined in Values.
	// CacheMaxSize will not be used if EnabledRegexMatching is set to false.
	CacheMaxSize int `mapstructure:"cache_max_size"`
	// InvertMatch indicates that values or regular expressions must not match against attribute values.
	// If InvertMatch is true and Values is equal to 'acme', all other values will be sampled except 'acme'.
	// Also, if the specified Key does not match on any resource or span attributes, data will be sampled.
	InvertMatch bool `mapstructure:"invert_match"`
}

// RateLimitingCfg holds the configurable settings to create a rate limiting
// sampling policy evaluator.
type RateLimitingCfg struct {
	// SpansPerSecond sets the limit on the maximum nuber of spans that can be processed each second.
	SpansPerSecond int64 `mapstructure:"spans_per_second"`
}

// SpanCountCfg holds the configurable settings to create a Span Count filter sampling policy
// sampling policy evaluator
type SpanCountCfg struct {
	// Minimum number of spans in a Trace
	MinSpans int32 `mapstructure:"min_spans"`
}

type PolicyFilterCfg struct {
	// values: AND | OR
	FilterOp string `mapstructure:"filter_op"`

	StringAttributeCfgs  []StringAttributeCfg  `mapstructure:"string_attributes"`
	NumericAttributeCfgs []NumericAttributeCfg `mapstructure:"numeric_attributes"`
	SpanCountCfg         `mapstructure:",squash"`
	StatusCodeCfg        `mapstructure:",squash"`
}

// PolicyCfg identifies policy rules in policy group
type PolicyCfg struct {
	// name of the policy
	Name string `mapstructure:"name"`

	// Type of the policy this will be used to match the proper configuration of the policy.
	Type PolicyType `mapstructure:"type"`

	// Set to true for sampling rule (root) and false for conditions
	Root bool `mapstructure:"root"`

	Priority int `mapstructure:"priority"`

	// sampling applied when  PolicyFilter matches
	ProbabilisticCfg `mapstructure:",squash"`

	// filter to activate policy
	PolicyFilterCfg `mapstructure:",squash"`

	SubPolicies []PolicyCfg `mapstructure:"sub_policies"`
}

// Config holds the configuration for tail-based sampling.
type Config struct {
	config.ProcessorSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct

	// DecisionWait is the desired wait time from the arrival of the first span of
	// trace until the decision about sampling it or not is evaluated.
	DecisionWait time.Duration `mapstructure:"decision_wait"`
	// NumTraces is the number of traces kept on memory. Typically most of the data
	// of a trace is released after a sampling decision is taken.
	NumTraces uint64 `mapstructure:"num_traces"`
	// ExpectedNewTracesPerSec sets the expected number of new traces sending to the tail sampling processor
	// per second. This helps with allocating data structures with closer to actual usage size.
	ExpectedNewTracesPerSec uint64 `mapstructure:"expected_new_traces_per_sec"`
	// PolicyCfgs sets the tail-based sampling policy which makes a sampling decision
	// for a given trace when requested.
	PolicyCfgs []PolicyCfg `mapstructure:"policies"`

	// read only version number (optional)
	Version int
}

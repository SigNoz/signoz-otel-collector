package metricreductionrule

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
)

// Suffixes of the flattened metric names that histograms and summaries are written as.
const (
	CountSuffix     = ".count"
	SumSuffix       = ".sum"
	MinSuffix       = ".min"
	MaxSuffix       = ".max"
	BucketSuffix    = ".bucket"
	QuantilesSuffix = ".quantile"
)

// ProtectedLabels can never be aggregated away: le/quantile are series identity,
// the dunder keys are baked into every fingerprint, and deployment.environment is
// a physical column and part of the sharding key.
var ProtectedLabels = map[string]struct{}{
	"le":                     {},
	"quantile":               {},
	"__name__":               {},
	"__temporality__":        {},
	"deployment.environment": {},
}

// ReductionRule is one compiled label drop/keep rule, keyed by the flattened
// metric name, e.g. http.server.duration.bucket.
type ReductionRule struct {
	// keys is the configured label set; keep picks the mode. keep=false drops
	// keys (rest kept); keep=true keeps only keys (rest dropped). protected
	// labels are kept either way.
	keys map[string]struct{}
	keep bool
	// effectiveFromUnixMilli is compared against the datapoint timestamp, not the
	// wall clock, so every replica reduces at the same data-time boundary.
	effectiveFromUnixMilli int64
}

func NewReductionRule(keys map[string]struct{}, keep bool, effectiveFromUnixMilli int64) *ReductionRule {
	return &ReductionRule{
		keys:                   keys,
		keep:                   keep,
		effectiveFromUnixMilli: effectiveFromUnixMilli,
	}
}

func (r *ReductionRule) Drop(key string) bool {
	_, listed := r.keys[key]
	if r.keep {
		if _, protected := ProtectedLabels[key]; protected {
			return false
		}
		return !listed
	}
	return listed
}

func (r *ReductionRule) AppliesAt(unixMilli int64) bool {
	return unixMilli >= r.effectiveFromUnixMilli
}

type RuleSet map[string]*ReductionRule

// Source hands out the latest RuleSet. Callers take one RuleSet per batch so a poll
// never splits a batch across two rule sets.
type Source interface {
	Enabled() bool
	Current() RuleSet
}

// FromHost returns the extension with this id, nil when id is nil, or an error when the
// extension is missing or is not a Source.
func FromHost(host component.Host, id *component.ID) (Source, error) {
	if id == nil {
		return nil, nil
	}
	ext, ok := host.GetExtensions()[*id]
	if !ok {
		return nil, fmt.Errorf("metric reduction rules extension %q not found", *id)
	}
	source, ok := ext.(Source)
	if !ok {
		return nil, fmt.Errorf("extension %q is not a metric reduction rules extension", *id)
	}
	return source, nil
}

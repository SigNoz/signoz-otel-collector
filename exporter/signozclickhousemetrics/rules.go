package signozclickhousemetrics

import (
	pkgfingerprint "github.com/SigNoz/signoz-otel-collector/internal/common/fingerprint"
	"github.com/SigNoz/signoz-otel-collector/pkg/metricreductionrule"
)

// reducer computes reduced fingerprints and series for one metric's datapoints
// under one rule; the reduced resource/scope fingerprints are computed once and
// reused for every datapoint.
type reducer struct {
	rule                *metricreductionrule.ReductionRule
	resourceFingerprint *pkgfingerprint.Fingerprint
	scopeFingerprint    *pkgfingerprint.Fingerprint

	reducedResource *pkgfingerprint.Fingerprint
	reducedScope    *pkgfingerprint.Fingerprint
	seen            map[uint64]struct{}

	// scratch backs the *reducedSeries handed to each call site, reused to avoid a
	// per-datapoint heap allocation.
	scratch reducedSeries
}

// firstSeen reports whether this reduced fingerprint is new within the batch.
// Many datapoints map to the same reduced series, so one row per batch is enough.
func (r *reducer) firstSeen(fingerprint uint64) bool {
	if r.seen == nil {
		r.seen = make(map[uint64]struct{})
	}
	if _, ok := r.seen[fingerprint]; ok {
		return false
	}
	r.seen[fingerprint] = struct{}{}
	return true
}

// reducedSeries carries what a call site needs to write the reduced sample and row.
type reducedSeries struct {
	fingerprint uint64
	point       *pkgfingerprint.Fingerprint
	scope       *pkgfingerprint.Fingerprint
	resource    *pkgfingerprint.Fingerprint
}

// ruleFor returns the active rule for the metric name, or nil.
func (c *clickhouseMetricsExporter) ruleFor(metricName string) *metricreductionrule.ReductionRule {
	if !c.reductionEnabled {
		return nil
	}
	return c.rules.Current()[metricName]
}

// newReducerFor returns a reducer for the metric name, or nil when reduction is
// disabled or no rule matches.
func (c *clickhouseMetricsExporter) newReducerFor(metricName string, resourceFingerprint, scopeFingerprint *pkgfingerprint.Fingerprint) *reducer {
	if !c.reductionEnabled {
		return nil
	}
	rule := c.ruleFor(metricName)
	if rule == nil {
		return nil
	}
	return &reducer{
		rule:                rule,
		resourceFingerprint: resourceFingerprint,
		scopeFingerprint:    scopeFingerprint,
	}
}

// reduce returns the reduced series for a datapoint fingerprint, or nil when the
// rule does not apply at the datapoint's timestamp (pre-epoch data stays unreduced
// and flows to the long-retention tables). nameWithSuffix is the stored metric
// name, e.g. metric.bucket.
func (r *reducer) reduce(pointFingerprint *pkgfingerprint.Fingerprint, nameWithSuffix string, unixMilli int64) *reducedSeries {
	if r == nil || !r.rule.AppliesAt(unixMilli) {
		return nil
	}
	if r.reducedResource == nil {
		r.reducedResource = r.resourceFingerprint.Reduced(pkgfingerprint.InitialOffset, r.rule.Drop)
		r.reducedScope = r.scopeFingerprint.Reduced(r.reducedResource.Hash(), r.rule.Drop)
	}
	reducedPoint := pointFingerprint.Reduced(r.reducedScope.Hash(), r.rule.Drop)
	r.scratch = reducedSeries{
		fingerprint: reducedPoint.HashWithName(nameWithSuffix),
		point:       reducedPoint,
		scope:       r.reducedScope,
		resource:    r.reducedResource,
	}
	return &r.scratch
}

func (r *reducedSeries) fingerprintOrZero() uint64 {
	if r == nil {
		return 0
	}
	return r.fingerprint
}

// reducedTsFrom builds the reduced series row from the raw one: same metadata, but
// the reduced fingerprint as identity and the remaining labels as labels/attrs.
func reducedTsFrom(raw *ts, reduced *reducedSeries) ts {
	pointMap := reduced.point.AttributesAsMap()
	scopeMap := reduced.scope.AttributesAsMap()
	resourceMap := reduced.resource.AttributesAsMap()
	return ts{
		env:                raw.env,
		temporality:        raw.temporality,
		metricName:         raw.metricName,
		description:        raw.description,
		unit:               raw.unit,
		typ:                raw.typ,
		isMonotonic:        raw.isMonotonic,
		fingerprint:        reduced.fingerprint,
		reducedFingerprint: reduced.fingerprint,
		isReduced:          true,
		unixMilli:          raw.unixMilli,
		labels:             pkgfingerprint.NewLabelsAsJSONString(raw.metricName, pointMap, scopeMap, resourceMap),
		attrs:              pointMap,
		scopeAttrs:         scopeMap,
		resourceAttrs:      resourceMap,
	}
}

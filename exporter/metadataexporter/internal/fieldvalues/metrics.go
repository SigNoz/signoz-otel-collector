package fieldvalues

import (
	"strings"
	"sync"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/SigNoz/signoz-otel-collector/internal/common/fingerprint"
)

const countSuffix = ".count"

type metricResource struct {
	hash uint64
	pairRange
}

// preparedSeries is one point: its series name and id, and its labels in
// pairs[lo:hi], the point labels first and then the scope labels.
type preparedSeries struct {
	name     string
	nameHash uint64
	resource int
	id       uint64
	ts       pcommon.Timestamp
	pairRange
}

type metricsInput struct {
	pairs     []pair
	resources []metricResource
	series    []preparedSeries
}

var metricsPool = sync.Pool{New: func() any { return &metricsInput{} }}

func getMetricsInput() *metricsInput {
	return metricsPool.Get().(*metricsInput)
}

func putMetricsInput(in *metricsInput) {
	if cap(in.pairs) > maxPooledPairs {
		return
	}
	clear(in.pairs)
	in.pairs = in.pairs[:0]
	in.resources = in.resources[:0]
	clear(in.series)
	in.series = in.series[:0]
	metricsPool.Put(in)
}

// addMetrics prepares one set per series, with the id of time_series_v4: the
// fingerprint chain of resource, scope and point attributes, with the metric
// name added last. Histograms and summaries write only their .count series,
// which has no le or quantile label. Metrics have no value limits.
func (in *metricsInput) addMetrics(md pmetric.Metrics) {
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		resourceFP := fingerprint.NewFingerprint(fingerprint.ResourceFingerprintType, fingerprint.InitialOffset, rm.Resource().Attributes(), nil)
		res := len(in.resources)
		lo := len(in.pairs)
		in.pairs = appendLabelPairs(in.pairs, contextResource, resourceFP.Attributes())
		in.resources = append(in.resources, metricResource{hash: resourceFP.Hash(), pairRange: pairRange{lo: lo, hi: len(in.pairs)}})
		sms := rm.ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			sm := sms.At(j)
			scopeFP := fingerprint.NewFingerprint(fingerprint.ScopeFingerprintType, resourceFP.Hash(), sm.Scope().Attributes(), map[string]string{
				"__scope.name__":       sm.Scope().Name(),
				"__scope.version__":    sm.Scope().Version(),
				"__scope.schema_url__": sm.SchemaUrl(),
			})
			scope := pairRange{lo: len(in.pairs)}
			in.pairs = appendLabelPairs(in.pairs, contextScope, scopeFP.Attributes())
			scope.hi = len(in.pairs)
			ms := sm.Metrics()
			for k := 0; k < ms.Len(); k++ {
				in.addMetric(ms.At(k), res, scopeFP.Hash(), scope)
			}
		}
	}
}

func (in *metricsInput) addMetric(m pmetric.Metric, res int, scopeHash uint64, scope pairRange) {
	name := m.Name()
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		dps := m.Gauge().DataPoints()
		extras := temporalityLabel(pmetric.AggregationTemporalityUnspecified)
		for i := 0; i < dps.Len(); i++ {
			in.addPoint(name, extras, dps.At(i).Attributes(), dps.At(i).Timestamp(), res, scopeHash, scope)
		}
	case pmetric.MetricTypeSum:
		dps := m.Sum().DataPoints()
		extras := temporalityLabel(m.Sum().AggregationTemporality())
		for i := 0; i < dps.Len(); i++ {
			in.addPoint(name, extras, dps.At(i).Attributes(), dps.At(i).Timestamp(), res, scopeHash, scope)
		}
	case pmetric.MetricTypeHistogram:
		dps := m.Histogram().DataPoints()
		extras := temporalityLabel(m.Histogram().AggregationTemporality())
		name += countSuffix
		for i := 0; i < dps.Len(); i++ {
			in.addPoint(name, extras, dps.At(i).Attributes(), dps.At(i).Timestamp(), res, scopeHash, scope)
		}
	case pmetric.MetricTypeExponentialHistogram:
		dps := m.ExponentialHistogram().DataPoints()
		extras := temporalityLabel(m.ExponentialHistogram().AggregationTemporality())
		name += countSuffix
		for i := 0; i < dps.Len(); i++ {
			in.addPoint(name, extras, dps.At(i).Attributes(), dps.At(i).Timestamp(), res, scopeHash, scope)
		}
	case pmetric.MetricTypeSummary:
		dps := m.Summary().DataPoints()
		extras := temporalityLabel(pmetric.AggregationTemporalityCumulative)
		name += countSuffix
		for i := 0; i < dps.Len(); i++ {
			in.addPoint(name, extras, dps.At(i).Attributes(), dps.At(i).Timestamp(), res, scopeHash, scope)
		}
	}
}

func temporalityLabel(t pmetric.AggregationTemporality) map[string]string {
	return map[string]string{"__temporality__": t.String()}
}

func (in *metricsInput) addPoint(name string, extras map[string]string, attrs pcommon.Map, ts pcommon.Timestamp, res int, scopeHash uint64, scope pairRange) {
	pointFP := fingerprint.NewFingerprint(fingerprint.PointFingerprintType, scopeHash, attrs, extras)
	lo := len(in.pairs)
	in.pairs = appendLabelPairs(in.pairs, contextAttribute, pointFP.Attributes())
	in.pairs = append(in.pairs, in.pairs[scope.lo:scope.hi]...)
	var nh uint64
	if n := len(in.series); n > 0 && in.series[n-1].name == name {
		nh = in.series[n-1].nameHash
	} else {
		nh = nameHash(name)
	}
	in.series = append(in.series, preparedSeries{
		name:      name,
		nameHash:  nh,
		resource:  res,
		id:        pointFP.HashWithName(name),
		ts:        ts,
		pairRange: pairRange{lo: lo, hi: len(in.pairs)},
	})
}

// appendLabelPairs gives the labels of a series as time_series_v4 keeps them,
// from the attributes of its fingerprint. Labels that the exporter adds
// itself, such as __temporality__, are part of the series id but write no
// rows. The pairs have no hashes: the series id identifies the set, and only
// labelKey needs the field, for new series.
func appendLabelPairs(dst []pair, ctx fieldContext, attrs fingerprint.Attributes) []pair {
	for _, a := range attrs {
		if strings.HasPrefix(a.Key, "__") && strings.HasSuffix(a.Key, "__") {
			continue
		}
		if a.Value.Val != "" {
			dst = append(dst, pair{ctx: ctx, typ: typeString, name: a.Key, str: a.Value.Val})
		}
	}
	return dst
}

func (b *batch) addSeries(in *metricsInput) {
	for i := range in.series {
		s := &in.series[i]
		res := &in.resources[s.resource]
		b.series(s, in.pairs[s.lo:s.hi], res.hash, in.pairs[res.lo:res.hi])
	}
}

func (b *batch) series(s *preparedSeries, labels []pair, rh uint64, resourcePairs []pair) {
	seen := b.seenMillis(s.ts, 0)
	sk := setKey(rh, s.id)
	if known, ahead := b.lookup(sk); !known {
		if b.room(classExact) {
			for i := range labels {
				b.emit(sk, s.name, &labels[i], rh, s.id, true, seen)
			}
			b.remember(sk, classExact)
			b.rememberLabels(s.nameHash, labels)
		} else {
			b.leaveOut(reasonCacheFull, 1)
			b.keepLabels(s, labels, rh, s.id, seen)
		}
	} else if ahead {
		b.emitAhead(sk, s.name, labels, rh, s.id, true)
	}
	rk := resourceKey(s.nameHash, rh)
	if rk == b.lastResourceKey {
		return
	}
	b.lastResourceKey = rk
	if known, ahead := b.lookup(rk); !known {
		if b.room(classExact) {
			for i := range resourcePairs {
				b.emit(rk, s.name, &resourcePairs[i], rh, resourceAttrsHash, true, seen)
			}
			b.remember(rk, classExact)
			b.rememberLabels(s.nameHash, resourcePairs)
		} else {
			b.leaveOut(reasonCacheFull, 1)
			b.keepLabels(s, resourcePairs, rh, resourceAttrsHash, seen)
		}
	} else if ahead {
		b.emitAhead(rk, s.name, resourcePairs, rh, resourceAttrsHash, true)
	}
}

// The keys of a metric are the field names in its rows, so each (metric,
// label) needs at least one row per window. rememberLabels marks the labels
// of a written series; keepLabels writes, from the reserve, one row for each
// label of a series that does not fit, if the label has no row in the window.
func (b *batch) rememberLabels(metricNameHash uint64, labels []pair) {
	for i := range labels {
		k := labelKey(metricNameHash, &labels[i])
		if known, _ := b.lookup(k); known {
			continue
		}
		if class, ok := b.classFor(); ok {
			b.remember(k, class)
		}
	}
}

func (b *batch) keepLabels(s *preparedSeries, labels []pair, resourceHash, attrsHash uint64, seen uint64) {
	for i := range labels {
		k := labelKey(s.nameHash, &labels[i])
		if known, ahead := b.lookup(k); known {
			if ahead {
				b.emitAhead(k, s.name, labels[i:i+1], resourceHash, attrsHash, true)
			}
			continue
		}
		if !b.room(classReserve) {
			b.leaveOut(reasonCacheFull, 1)
			continue
		}
		b.emit(k, s.name, &labels[i], resourceHash, attrsHash, true, seen)
		b.remember(k, classReserve)
	}
}

package fieldvalues

import (
	"strings"
	"sync"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/SigNoz/signoz-otel-collector/internal/common/fingerprint"
)

const countSuffix = ".count"

// linkField is the field of the link rows: a link row has no metric name, the
// resource field __name__ with the metric name as its value, and the
// resource hash. It links a metric to a resource whose rows are written once,
// with no metric name.
const linkField = "__name__"

// metricResource is a resource of the input. keys identifies the names of its
// fields, for the key rows of its metrics.
type metricResource struct {
	hash uint64
	keys uint64
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
		in.pairs = appendResourcePairs(in.pairs, resourceFP.Attributes())
		var keys uint64
		for i := lo; i < len(in.pairs); i++ {
			keys += uint64(in.pairs[i].field)
		}
		in.resources = append(in.resources, metricResource{hash: resourceFP.Hash(), keys: mix64(keys), pairRange: pairRange{lo: lo, hi: len(in.pairs)}})
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

// appendResourcePairs gives the resource fields of a series, with their
// hashes for the key rows.
func appendResourcePairs(dst []pair, attrs fingerprint.Attributes) []pair {
	for _, a := range attrs {
		if strings.HasPrefix(a.Key, "__") && strings.HasSuffix(a.Key, "__") {
			continue
		}
		if a.Value.Val != "" {
			dst = append(dst, stringPair(contextResource, a.Key, a.Value.Val))
		}
	}
	return dst
}

func (b *batch) addSeries(in *metricsInput) {
	for i := range in.series {
		s := &in.series[i]
		res := &in.resources[s.resource]
		b.series(s, in.pairs[s.lo:s.hi], res, in.pairs[res.lo:res.hi])
	}
}

// series writes the rows of one point: the series set, the resource rows once
// per resource with no metric name, a link row per metric and resource, and
// a key row per metric and resource field, so that the keys of a metric are
// complete. The points of a resource come together, so the resource and the
// link are looked up once for a run of points.
func (b *batch) series(s *preparedSeries, labels []pair, res *metricResource, resourcePairs []pair) {
	seen := b.seenMillis(s.ts, 0)
	rh := res.hash
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
	if s.resource != b.lastResource {
		b.lastResource = s.resource
		b.lastLink = 0
		rk := resourceKey(emptyNameHash, rh)
		if known, ahead := b.lookup(rk); !known {
			if class, ok := b.classFor(); ok {
				for i := range resourcePairs {
					b.emit(rk, "", &resourcePairs[i], rh, resourceAttrsHash, true, seen)
				}
				b.remember(rk, class)
			} else {
				b.leaveOut(reasonCacheFull, len(resourcePairs))
			}
		} else if ahead {
			b.emitAhead(rk, "", resourcePairs, rh, resourceAttrsHash, true)
		}
	}
	lk := linkKey(s.nameHash, rh)
	if lk == b.lastLink {
		return
	}
	b.lastLink = lk
	link := stringPair(contextResource, linkField, s.name)
	if known, ahead := b.lookup(lk); !known {
		if class, ok := b.classFor(); ok {
			b.emit(lk, "", &link, rh, resourceAttrsHash, false, seen)
			b.remember(lk, class)
		} else {
			b.leaveOut(reasonCacheFull, 1)
		}
	} else if ahead {
		b.emitAhead(lk, "", []pair{link}, rh, resourceAttrsHash, false)
	}
	b.resourceKeys(s, res, resourcePairs, seen)
}

// resourceKeys writes a key row for each resource field of a metric that has
// none in the window. A key row has the metric name, the field and no value.
// It is checked once per batch for a metric and the field names of a
// resource, as most resources of a metric have the same fields.
func (b *batch) resourceKeys(s *preparedSeries, res *metricResource, resourcePairs []pair, seen uint64) {
	checked := mix64(s.nameHash ^ res.keys)
	if _, ok := b.keysChecked[checked]; ok {
		return
	}
	b.keysChecked[checked] = struct{}{}
	for i := range resourcePairs {
		k := labelKey(s.nameHash, resourcePairs[i].field)
		key := pair{ctx: contextResource, typ: typeString, name: resourcePairs[i].name}
		if known, ahead := b.lookup(k); !known {
			if class, ok := b.classFor(); ok {
				b.emit(k, s.name, &key, res.hash, resourceAttrsHash, false, seen)
				b.remember(k, class)
			} else {
				b.leaveOut(reasonCacheFull, 1)
			}
		} else if ahead {
			b.emitAhead(k, s.name, []pair{key}, res.hash, resourceAttrsHash, false)
		}
	}
}

// The keys of a metric are the field names in its rows, so each (metric,
// label) needs at least one row per window. rememberLabels marks the labels
// of a written series; keepLabels writes, from the reserve, one row for each
// label of a series that does not fit, if the label has no row in the window.
func (b *batch) rememberLabels(metricNameHash uint64, labels []pair) {
	for i := range labels {
		k := labelKey(metricNameHash, fieldIDOf(labels[i].ctx, labels[i].name))
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
		k := labelKey(s.nameHash, fieldIDOf(labels[i].ctx, labels[i].name))
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

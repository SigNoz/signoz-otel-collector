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
	keys uint64
	pairRange
}

// preparedSeries is one point: its series name and id, and its labels in
// pairs[lo:hi], the point labels first and then the scope labels. The id does
// not include the resource: under the writer lock, the series joins the
// identity of its resource, which the value rules can make coarse.
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

// addMetrics prepares one set per series, with an id from the fingerprint
// chain of scope and point attributes, with the metric name added last.
// Histograms and summaries write only their .count series, which has no le or
// quantile label.
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
		in.resources = append(in.resources, metricResource{keys: mix64(keys), pairRange: pairRange{lo: lo, hi: len(in.pairs)}})
		sms := rm.ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			sm := sms.At(j)
			scopeFP := fingerprint.NewFingerprint(fingerprint.ScopeFingerprintType, fingerprint.InitialOffset, sm.Scope().Attributes(), map[string]string{
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
// a new series needs them.
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

// series writes the rows of one point: the resource rows with no metric name,
// as for logs and traces, the series set, a link row per metric and resource,
// and a key row per metric and resource field, so that the keys of a metric
// are complete. The resource is its identity: when a resource field is over
// the limit, the series of the resources that differ only in that field are
// one series. The points of a resource come together, so the resource and the
// link are looked up once for a run of points.
func (b *batch) series(s *preparedSeries, labels []pair, res *metricResource, resourcePairs []pair) {
	seen := b.seenMillis(s.ts, 0)
	if s.resource != b.lastResource {
		b.lastResource = s.resource
		b.lastResourceHash = b.resourceRows(resourcePairs, seen)
		b.lastLink = 0
	}
	rh := b.lastResourceHash
	sk := setKey(rh, s.id)
	if !b.lookup(sk) {
		b.newSeries(s, labels, rh, sk, seen)
	}
	lk := linkKey(s.nameHash, rh)
	if lk == b.lastLink {
		return
	}
	b.lastLink = lk
	link := stringPair(contextResource, linkField, s.name)
	if !b.lookup(lk) {
		if class, ok := b.classFor(); ok {
			b.emit(lk, "", &link, rh, resourceAttrsHash, false, seen)
			b.remember(lk, class)
		} else {
			b.leaveOut(reasonCacheFull, 1)
		}
		b.resourceKeys(s, res, resourcePairs)
	}
}

// newSeries writes a series that the window does not know. Its labels follow
// the value rules of logs and traces. When all labels stay in the set, the
// set is the series. When a label leaves the set, the set is the labels that
// stay, and the series is not cached: a label with a value per point would
// fill the cache with series.
func (b *batch) newSeries(s *preparedSeries, labels []pair, rh, sk, seen uint64) {
	in, samples := b.in[:0], b.sampled[:0]
	for i := range labels {
		p := &labels[i]
		if p.field == 0 {
			p.setHashes()
		}
		switch b.decide(p, false, s.nameHash, s.name) {
		case modeInHash:
			in = append(in, *p)
		case modeSample:
			samples = append(samples, *p)
		}
	}
	b.in, b.sampled = in, samples
	if len(in) == len(labels) {
		b.seriesSet(s, in, rh, sk, s.id, seen)
		return
	}
	ah := mix64(setHash(in) ^ s.nameHash*0x9e3779b97f4a7c15)
	if ah <= overflowAttrsHash {
		ah += 2
	}
	ck := setKey(rh, ah)
	if !b.lookup(ck) {
		b.seriesSet(s, in, rh, ck, ah, seen)
	}
	for i := range samples {
		b.emit(0, s.name, &samples[i], rh, ah, false, seen)
	}
}

// seriesSet writes the rows of a new set of a metric: the series, or the
// labels of a series that stay in the set.
func (b *batch) seriesSet(s *preparedSeries, in []pair, rh, k, ah, seen uint64) {
	if !b.room(classExact) {
		b.leaveOut(reasonCacheFull, 1)
		return
	}
	for i := range in {
		b.emit(k, s.name, &in[i], rh, ah, true, seen)
	}
	b.remember(k, classExact)
}

// resourceKeys records the resource fields of a metric as keys of the metric,
// when the link of the metric to a resource is new. It runs once per batch
// for a metric and the field names of a resource, as most resources of a
// metric have the same fields.
func (b *batch) resourceKeys(s *preparedSeries, res *metricResource, resourcePairs []pair) {
	checked := mix64(s.nameHash ^ res.keys)
	if _, ok := b.keysChecked[checked]; ok {
		return
	}
	b.keysChecked[checked] = struct{}{}
	for i := range resourcePairs {
		b.state.keys.see(&resourcePairs[i], s.nameHash, s.name)
	}
}

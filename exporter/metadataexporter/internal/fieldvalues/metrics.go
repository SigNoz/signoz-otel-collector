package fieldvalues

import (
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/SigNoz/signoz-otel-collector/internal/common/fingerprint"
)

const countSuffix = ".count"

// addMetrics writes one set per series, with the id of time_series_v4: the
// fingerprint chain of resource, scope and point attributes, with the metric
// name added last. Histograms and summaries write only their .count series,
// which has no le or quantile label. Metrics have no value limits.
func (b *batch) addMetrics(md pmetric.Metrics) {
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		resourceFP := fingerprint.NewFingerprint(fingerprint.ResourceFingerprintType, fingerprint.InitialOffset, rm.Resource().Attributes(), map[string]string{})
		rh := resourceFP.Hash()
		resourcePairs := labelPairs(contextResource, rm.Resource().Attributes())
		sms := rm.ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			sm := sms.At(j)
			scopeFP := fingerprint.NewFingerprint(fingerprint.ScopeFingerprintType, rh, sm.Scope().Attributes(), map[string]string{
				"__scope.name__":       sm.Scope().Name(),
				"__scope.version__":    sm.Scope().Version(),
				"__scope.schema_url__": sm.SchemaUrl(),
			})
			s := seriesScope{
				resourceHash:  rh,
				scopeHash:     scopeFP.Hash(),
				resourcePairs: resourcePairs,
				scopePairs:    labelPairs(contextScope, sm.Scope().Attributes()),
			}
			ms := sm.Metrics()
			for k := 0; k < ms.Len(); k++ {
				b.addMetric(ms.At(k), s)
			}
		}
	}
}

type seriesScope struct {
	resourceHash  uint64
	scopeHash     uint64
	resourcePairs []pair
	scopePairs    []pair
}

func (b *batch) addMetric(m pmetric.Metric, s seriesScope) {
	name := m.Name()
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		dps := m.Gauge().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			b.series(name, pmetric.AggregationTemporalityUnspecified, dps.At(i).Attributes(), dps.At(i).Timestamp(), s)
		}
	case pmetric.MetricTypeSum:
		dps := m.Sum().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			b.series(name, m.Sum().AggregationTemporality(), dps.At(i).Attributes(), dps.At(i).Timestamp(), s)
		}
	case pmetric.MetricTypeHistogram:
		dps := m.Histogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			b.series(name+countSuffix, m.Histogram().AggregationTemporality(), dps.At(i).Attributes(), dps.At(i).Timestamp(), s)
		}
	case pmetric.MetricTypeExponentialHistogram:
		dps := m.ExponentialHistogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			b.series(name+countSuffix, m.ExponentialHistogram().AggregationTemporality(), dps.At(i).Attributes(), dps.At(i).Timestamp(), s)
		}
	case pmetric.MetricTypeSummary:
		dps := m.Summary().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			b.series(name+countSuffix, pmetric.AggregationTemporalityCumulative, dps.At(i).Attributes(), dps.At(i).Timestamp(), s)
		}
	}
}

func (b *batch) series(seriesName string, temporality pmetric.AggregationTemporality, attrs pcommon.Map, ts pcommon.Timestamp, s seriesScope) {
	pointFP := fingerprint.NewFingerprint(fingerprint.PointFingerprintType, s.scopeHash, attrs, map[string]string{
		"__temporality__": temporality.String(),
	})
	seriesID := pointFP.HashWithName(seriesName)
	seen := b.seenMillis(ts, 0)

	if k := setKey(s.resourceHash, seriesID); !b.known(k) {
		labels := append(labelPairs(contextAttribute, attrs), s.scopePairs...)
		if b.room(classExact) {
			for _, p := range labels {
				b.emit(k, seriesName, p, s.resourceHash, seriesID, true, seen)
			}
			b.remember(k, classExact)
			b.rememberLabels(seriesName, labels)
		} else {
			b.leaveOut(reasonCacheFull, 1)
			b.keepLabels(seriesName, labels, s.resourceHash, seriesID, seen)
		}
	}
	if k := resourceKey(seriesName, s.resourceHash); !b.known(k) {
		if b.room(classExact) {
			for _, p := range s.resourcePairs {
				b.emit(k, seriesName, p, s.resourceHash, resourceAttrsHash, true, seen)
			}
			b.remember(k, classExact)
			b.rememberLabels(seriesName, s.resourcePairs)
		} else {
			b.leaveOut(reasonCacheFull, 1)
			b.keepLabels(seriesName, s.resourcePairs, s.resourceHash, resourceAttrsHash, seen)
		}
	}
}

// The keys of a metric are the field names in its rows, so each (metric,
// label) needs at least one row per day. rememberLabels marks the labels of a
// written series; keepLabels writes, from the reserve, one row for each label
// of a series that does not fit, if the label has no row today.
func (b *batch) rememberLabels(metricName string, labels []pair) {
	for _, p := range labels {
		k := labelKey(metricName, p)
		if b.known(k) {
			continue
		}
		if class, ok := b.classFor(); ok {
			b.remember(k, class)
		}
	}
}

func (b *batch) keepLabels(metricName string, labels []pair, resourceHash, attrsHash uint64, seen uint64) {
	for _, p := range labels {
		k := labelKey(metricName, p)
		if b.known(k) {
			continue
		}
		if !b.room(classReserve) {
			b.leaveOut(reasonCacheFull, 1)
			continue
		}
		b.emit(k, metricName, p, resourceHash, attrsHash, true, seen)
		b.remember(k, classReserve)
	}
}

func labelKey(metricName string, p pair) uint64 {
	h := hashByte(fnvOffset, 'K')
	h = hashString(h, metricName)
	h = hashByte(h, separatorByte)
	h = hashByte(h, byte(p.ctx))
	return mix64(hashString(h, p.name))
}

// labelPairs gives the labels of a series as strings, as time_series_v4 keeps
// them. Labels that the exporter adds itself, such as __temporality__, are
// part of the series id but write no rows.
func labelPairs(ctx fieldContext, m pcommon.Map) []pair {
	pairs := make([]pair, 0, m.Len())
	m.Range(func(k string, v pcommon.Value) bool {
		if strings.HasPrefix(k, "__") && strings.HasSuffix(k, "__") {
			return true
		}
		if value := v.AsString(); value != "" {
			pairs = append(pairs, stringPair(ctx, k, value))
		}
		return true
	})
	return pairs
}

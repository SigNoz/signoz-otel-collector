package signozclickhousemetrics

import (
	"github.com/SigNoz/signoz-otel-collector/exporter/signozclickhousemetrics/internal"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type batch struct {
	samples  []*sample
	expHist  []*exponentialHistogramSample
	ts       []*ts
	metadata []*metadata

	metaSeen map[string]struct{}
}

func newBatch() *batch {
	return &batch{
		samples:  make([]*sample, 0),
		expHist:  make([]*exponentialHistogramSample, 0),
		ts:       make([]*ts, 0),
		metadata: make([]*metadata, 0),
		metaSeen: make(map[string]struct{}),
	}
}

func (b *batch) addMetadata(name, desc, unit string, typ pmetric.MetricType, temporality pmetric.AggregationTemporality, isMonotonic bool, fingerprint *internal.Fingerprint) {
	for key, value := range fingerprint.Attributes() {
		// there should never be a conflicting key (either with resource, scope, or point attributes) in metrics
		// it breaks the fingerprinting, we assume this will never happen
		// even if it does, we will not handle it on our end (because we can't reliably which should take
		// precedence), the user should be responsible for ensuring no conflicting keys in their metrics
		if _, ok := b.metaSeen[key]; ok {
			continue
		}
		b.metaSeen[key] = struct{}{}
		b.metadata = append(b.metadata, &metadata{
			metricName:      name,
			temporality:     temporality,
			description:     desc,
			unit:            unit,
			typ:             typ,
			isMonotonic:     isMonotonic,
			attrName:        key,
			attrType:        fingerprint.Type().String(),
			attrDatatype:    value.DataType,
			attrStringValue: value.Val,
		})
	}
}

func (b *batch) addSample(sample *sample) {
	b.samples = append(b.samples, sample)
}

func (b *batch) addTs(ts *ts) {
	b.ts = append(b.ts, ts)
}

func (b *batch) addExpHist(expHist *exponentialHistogramSample) {
	b.expHist = append(b.expHist, expHist)
}

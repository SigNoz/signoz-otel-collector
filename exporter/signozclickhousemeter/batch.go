package signozclickhousemeter

import (
	pkgfingerprint "github.com/SigNoz/signoz-otel-collector/internal/common/fingerprint"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type batch struct {
	samples  []*sample
	metadata []*metadata

	metaSeen map[string]struct{}
}

func newBatch() *batch {
	return &batch{
		samples:  make([]*sample, 0),
		metadata: make([]*metadata, 0),
		metaSeen: make(map[string]struct{}),
	}
}

func (b *batch) addMetadata(name, desc, unit string, typ pmetric.MetricType, temporality pmetric.AggregationTemporality, isMonotonic bool, fingerprint *pkgfingerprint.Fingerprint) {
	for key, value := range fingerprint.Attributes() {
		seenKey := key + name
		if key == "le" {
			seenKey += value.Val
		}
		// there should never be a conflicting key (either with resource, scope, or point attributes) in metrics
		// it breaks the fingerprinting, we assume this will never happen
		// even if it does, we will not handle it on our end (because we can't reliably which should take
		// precedence), the user should be responsible for ensuring no conflicting keys in their metrics
		if _, ok := b.metaSeen[seenKey]; ok {
			continue
		}
		b.metaSeen[seenKey] = struct{}{}
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

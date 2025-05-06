package pmetricsgen

type Count struct {
	GaugeMetricsCount                       int
	GaugeDataPointCount                     int
	GaugeNanValuesCount                     int
	GaugeNoRecordCount                      int
	GaugePointAttributeCount                int
	SumMetricsCount                         int
	SumDataPointCount                       int
	SumNoRecordCount                        int
	SumNanValuesCount                       int
	SumPointAttributeCount                  int
	HistogramMetricsCount                   int
	HistogramDataPointCount                 int
	HistogramBucketCount                    int
	HistogramNanValuesCount                 int
	HistogramNoRecordCount                  int
	HistogramPointAttributeCount            int
	ExponentialHistogramMetricsCount        int
	ExponentialHistogramDataPointCount      int
	ExponentialHistogramBucketCount         int
	ExponentialHistogramNanValuesCount      int
	ExponentialHistogramNoRecordCount       int
	ExponentialHistogramPointAttributeCount int
	SummaryMetricsCount                     int
	SummaryDataPointCount                   int
	SummaryQuantileCount                    int
	SummaryNanValuesCount                   int
	SummaryNoRecordCount                    int
	SummaryPointAttributeCount              int
}

type generationOptions struct {
	resourceAttributeCount       int
	resourceAttributeStringValue string
	scopeAttributeCount          int
	scopeAttributeStringValue    string
	count                        Count
	attributes                   map[string]any
}

type GenerationOption func(*generationOptions)

func WithResourceAttributeCount(i int) GenerationOption {
	return func(o *generationOptions) {
		o.resourceAttributeCount = i
	}
}

func WithResourceAttributeStringValue(s string) GenerationOption {
	return func(o *generationOptions) {
		o.resourceAttributeStringValue = s
	}
}

func WithScopeAttributeCount(i int) GenerationOption {
	return func(o *generationOptions) {
		o.scopeAttributeCount = i
	}
}

func WithScopeAttributeStringValue(s string) GenerationOption {
	return func(o *generationOptions) {
		o.scopeAttributeStringValue = s
	}
}

func WithCount(s Count) GenerationOption {
	return func(o *generationOptions) {
		o.count = s
	}
}

func WithAttributes(m map[string]any) GenerationOption {
	return func(o *generationOptions) {
		o.attributes = m
	}
}

package v1

import (
	"github.com/SigNoz/signoz-otel-collector/pkg/metering"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type metrics struct {
	Logger *zap.Logger
	Sizer  metering.Sizer
}

func NewMetrics(logger *zap.Logger) metering.Metrics {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &metrics{
		Logger: logger,
		Sizer:  metering.NewJSONSizer(logger),
	}
}

func (meter *metrics) Size(md pmetric.Metrics) int {
	// Note: We will never use this function unless our billing mechanism changes
	// The metrics limits should have been samples count based, however, we rolled out with size
	// and many have a guestimated and set the size limits. So we will keep this as pmetric json size for now
	bytes, err := (&pmetric.JSONMarshaler{}).MarshalMetrics(md)
	if err != nil {
		meter.Logger.Error("cannot marshal metrics", zap.Error(err))
		return 0
	}

	return len(bytes)
}

func (meter *metrics) Count(md pmetric.Metrics) int {
	count := 0
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		resourceMetric := md.ResourceMetrics().At(i)

		for j := 0; j < resourceMetric.ScopeMetrics().Len(); j++ {
			scopeMetrics := resourceMetric.ScopeMetrics().At(j)

			for k := 0; k < scopeMetrics.Metrics().Len(); k++ {
				metric := scopeMetrics.Metrics().At(k)

				switch metric.Type() {
				case pmetric.MetricTypeGauge:
					count += metric.Gauge().DataPoints().Len()
				case pmetric.MetricTypeSum:
					count += metric.Sum().DataPoints().Len()
				case pmetric.MetricTypeHistogram:
					subCount := 0
					// each bucket is treated as separate sample
					for i := 0; i < metric.Histogram().DataPoints().Len(); i++ {
						subCount += metric.Histogram().DataPoints().At(i).BucketCounts().Len()
					}
					count += subCount
				case pmetric.MetricTypeSummary:
					subCount := 0
					for i := 0; i < metric.Summary().DataPoints().Len(); i++ {
						subCount += metric.Summary().DataPoints().At(i).QuantileValues().Len()
					}
					count += subCount
				case pmetric.MetricTypeExponentialHistogram:
					// we haven't enabled this metric type on Cloud since we don't know how to bill this
					// If we use the following logic, it will explode the samples count
					// However, for the sake of completeness, following the same logic as ExplicitBucketHistogram
					// TODO(srikanthccv): Update this when we support this metric type on Cloud
					subCount := 0
					for i := 0; i < metric.ExponentialHistogram().DataPoints().Len(); i++ {
						// each bucket of positive and negative is treated as separate sample
						subCount += metric.ExponentialHistogram().DataPoints().At(i).Negative().BucketCounts().Len() +
							metric.ExponentialHistogram().DataPoints().At(i).Positive().BucketCounts().Len()
					}
					count += subCount
				}
			}
		}
	}
	return count
}

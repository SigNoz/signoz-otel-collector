package v1

import (
	"math"
	"regexp"

	"github.com/SigNoz/signoz-otel-collector/pkg/metering"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

var (
	excludeRegex = regexp.MustCompile("^(signoz|otelcol).*")
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

func (meter *metrics) SizePerResource(rmd pmetric.ResourceMetrics) int {
	// check what can be done here
	return 0
}

func (meter *metrics) Count(md pmetric.Metrics) int {
	count := 0
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		resourceMetric := md.ResourceMetrics().At(i)
		count += meter.CountPerResource(resourceMetric)
	}
	return count
}

func (meter *metrics) CountPerResource(rmd pmetric.ResourceMetrics) int {
	count := 0

	for j := 0; j < rmd.ScopeMetrics().Len(); j++ {
		scopeMetrics := rmd.ScopeMetrics().At(j)

		for k := 0; k < scopeMetrics.Metrics().Len(); k++ {
			metric := scopeMetrics.Metrics().At(k)
			// if the metric satisfies the excluded regex then skip the metric
			if excludeRegex.MatchString(metric.Name()) {
				continue
			}

			switch metric.Type() {
			case pmetric.MetricTypeGauge:
				count += meter.countNumberSamples(metric.Gauge().DataPoints())
			case pmetric.MetricTypeSum:
				count += meter.countNumberSamples(metric.Sum().DataPoints())
			case pmetric.MetricTypeHistogram:
				count += meter.countHistogramSamples(metric.Histogram().DataPoints())
			case pmetric.MetricTypeSummary:
				count += meter.countSummarySamples(metric.Summary().DataPoints())
			case pmetric.MetricTypeExponentialHistogram:
				// Intentionally not counted. Exp histograms aren't billable on Cloud yet,
				// and counting each bucket as a sample would explode the count.
				// Kept consistent with the signozclickhousemetrics exporter's writeExpHist,
				// which also does not count these toward usage.
				// TODO(srikanthccv): revisit when exp histograms are supported on Cloud.
			}
		}
	}

	return count
}

func (meter *metrics) countNumberSamples(dps pmetric.NumberDataPointSlice) int {
	count := 0
	for i := range dps.Len() {
		if !math.IsNaN(dps.At(i).DoubleValue()) {
			count++
		}
	}
	return count
}

func (meter *metrics) countHistogramSamples(dps pmetric.HistogramDataPointSlice) int {
	count := 0
	for i := range dps.Len() {
		dp := dps.At(i)
		if math.IsNaN(dp.Sum()) || math.IsNaN(dp.Min()) || math.IsNaN(dp.Max()) {
			continue
		}
		// each bucket is treated as separate sample
		count += dp.BucketCounts().Len()
		count += 1 // count metric
		if dp.HasSum() {
			count++
		}
		if dp.HasMin() {
			count++
		}
		if dp.HasMax() {
			count++
		}
	}
	return count
}

func (meter *metrics) countSummarySamples(dps pmetric.SummaryDataPointSlice) int {
	count := 0
	for i := range dps.Len() {
		dp := dps.At(i)
		quantiles := dp.QuantileValues()
		hasNaN := math.IsNaN(dp.Sum())
		for j := range quantiles.Len() {
			if math.IsNaN(quantiles.At(j).Value()) {
				hasNaN = true
				break
			}
		}
		if hasNaN {
			continue
		}
		count += quantiles.Len()
		count += 2 // count,sum metrics
	}
	return count
}

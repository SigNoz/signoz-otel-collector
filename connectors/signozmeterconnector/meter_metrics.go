package signozmeterconnector

import (
	"sync"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

type meterMetric struct {
	spanCount            int
	spanSize             int
	metricDataPointCount int
	metricDataPointSize  int
	logCount             int
	logSize              int
	attrs                pcommon.Map
}

type aggregatedMeterMetrics struct {
	meterMetrics map[[16]byte]*meterMetric
	sync.Mutex
}

func newAggregatedMeterMetrics() *aggregatedMeterMetrics {
	return &aggregatedMeterMetrics{
		meterMetrics: map[[16]byte]*meterMetric{},
	}
}

func (agm *aggregatedMeterMetrics) UpdateMetricDataPointsMeterMetrics(attrs pcommon.Map, count, size int) {
	key := pdatautil.MapHash(attrs)
	agm.Lock()
	defer agm.Unlock()

	meterMetrics, ok := agm.meterMetrics[key]
	if !ok {
		meterMetrics = &meterMetric{attrs: attrs}
		agm.meterMetrics[key] = meterMetrics
	}
	meterMetrics.metricDataPointCount += count
	meterMetrics.metricDataPointSize += size
}

func (agm *aggregatedMeterMetrics) UpdateSpanMeterMetrics(attrs pcommon.Map, count, size int) {
	key := pdatautil.MapHash(attrs)
	agm.Lock()
	defer agm.Unlock()

	meterMetrics, ok := agm.meterMetrics[key]
	if !ok {
		meterMetrics = &meterMetric{attrs: attrs}
		agm.meterMetrics[key] = meterMetrics
	}
	meterMetrics.spanCount += count
	meterMetrics.spanSize += size
}

func (agm *aggregatedMeterMetrics) UpdateLogMeterMetrics(attrs pcommon.Map, count, size int) {
	key := pdatautil.MapHash(attrs)
	agm.Lock()
	defer agm.Unlock()

	meterMetrics, ok := agm.meterMetrics[key]
	if !ok {
		meterMetrics = &meterMetric{attrs: attrs}
		agm.meterMetrics[key] = meterMetrics
	}
	meterMetrics.logCount += count
	meterMetrics.logSize += size
}

func (agm *aggregatedMeterMetrics) Purge() {
	agm.meterMetrics = map[[16]byte]*meterMetric{}
}

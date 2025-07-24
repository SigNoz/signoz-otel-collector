package signozmeterconnector

import (
	"sync"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

type meterMetrics struct {
	spanCount            int
	spanSize             int
	metricDataPointCount int
	metricDataPointSize  int
	logCount             int
	logSize              int
	attrs                pcommon.Map
}

type aggregatedMeterMetrics struct {
	meterMetrics map[[16]byte]*meterMetrics
	sync.Mutex
}

func newMeterMetrics(attrs pcommon.Map) *meterMetrics {
	return &meterMetrics{attrs: attrs}
}

func (meterMetrics *meterMetrics) UpdateMetricDataPointMeterMetrics(count, size int) {
	meterMetrics.metricDataPointCount = count
	meterMetrics.metricDataPointSize = size
}

func (meterMetrics *meterMetrics) UpdateSpanMeterMetrics(count, size int) {
	meterMetrics.spanCount = count
	meterMetrics.spanSize = size
}

func (meterMetrics *meterMetrics) UpdateLogMeterMetrics(count, size int) {
	meterMetrics.logCount = count
	meterMetrics.logSize = size
}

func newAggregatedMeterMetrics() *aggregatedMeterMetrics {
	return &aggregatedMeterMetrics{
		meterMetrics: map[[16]byte]*meterMetrics{},
	}
}

func (agm *aggregatedMeterMetrics) UpdateMetricDataPointsMeterMetrics(attrs pcommon.Map, count, size int) {
	key := pdatautil.MapHash(attrs)
	agm.Lock()
	defer agm.Unlock()

	meterMetrics, ok := agm.meterMetrics[key]
	if !ok {
		meterMetrics = newMeterMetrics(attrs)
		agm.meterMetrics[key] = meterMetrics
	}
	meterMetrics.UpdateMetricDataPointMeterMetrics(count, size)
}

func (agm *aggregatedMeterMetrics) UpdateSpanMeterMetrics(attrs pcommon.Map, count, size int) {
	key := pdatautil.MapHash(attrs)
	agm.Lock()
	defer agm.Unlock()

	meterMetrics, ok := agm.meterMetrics[key]
	if !ok {
		meterMetrics = newMeterMetrics(attrs)
		agm.meterMetrics[key] = meterMetrics
	}
	meterMetrics.UpdateSpanMeterMetrics(count, size)
}

func (agm *aggregatedMeterMetrics) UpdateLogMeterMetrics(attrs pcommon.Map, count, size int) {
	key := pdatautil.MapHash(attrs)
	agm.Lock()
	defer agm.Unlock()

	meterMetrics, ok := agm.meterMetrics[key]
	if !ok {
		meterMetrics = newMeterMetrics(attrs)
		agm.meterMetrics[key] = meterMetrics
	}
	meterMetrics.UpdateLogMeterMetrics(count, size)
}

func (agm *aggregatedMeterMetrics) Purge() {
	agm.meterMetrics = map[[16]byte]*meterMetrics{}
}

// Code generated by mdatagen. DO NOT EDIT.

package clickhouselogsexporter

import (
	"go.uber.org/goleak"
	"testing"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m, goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"), goleak.IgnoreTopFunction("go.opencensus.io/metric/metricexport.(*IntervalReader).startInternal"))
}

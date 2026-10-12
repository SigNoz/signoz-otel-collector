package signozmeterconnector

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap/zaptest"
)

func BenchmarkBuildMetrics(b *testing.B) {
	connector, err := newConnector(zaptest.NewLogger(b), connectortest.NewNopSettings(typ), &Config{Dimensions: []Dimension{{Name: "host.name"}}, MetricsFlushInterval: time.Hour})
	require.NoError(b, err)

	for i := 0; i < 10000; i++ {
		m := pcommon.NewMap()
		m.PutStr("host.name", "host-"+strconv.Itoa(i))
		connector.aggregatedMeterMetrics.UpdateLogMeterMetrics(m, 1, 100)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_ = connector.buildMetrics()
	}
}

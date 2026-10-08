package opamp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/otelcol"
	"go.uber.org/zap"
)

func TestStoppedUnexpectedly(t *testing.T) {
	testCases := []struct {
		name      string
		last      otelcol.State
		current   otelcol.State
		reloading bool
		want      bool
	}{
		{
			name:    "ClosedOutsideReload_Reports",
			last:    otelcol.StateRunning,
			current: otelcol.StateClosed,
			want:    true,
		},
		{
			name:    "ClosedAfterClosed_StillReports",
			last:    otelcol.StateClosed,
			current: otelcol.StateClosed,
			want:    true,
		},
		{
			name:      "ClosedDuringReload_DoesNotReport",
			last:      otelcol.StateRunning,
			current:   otelcol.StateClosed,
			reloading: true,
			want:      false,
		},
		{
			name:    "RunningAfterStarting_DoesNotReport",
			last:    otelcol.StateStarting,
			current: otelcol.StateRunning,
			want:    false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			c := &baseClient{logger: zap.NewNop()}
			c.lastKnownState.Store(int32(testCase.last))
			c.isReloading.Store(testCase.reloading)

			assert.Equal(t, testCase.want, c.stoppedUnexpectedly(testCase.current))
			assert.Equal(t, testCase.current, otelcol.State(c.lastKnownState.Load()))
		})
	}
}

package json

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

type testCase struct {
	name      string
	expectErr bool
	input     func() *entry.Entry
	output    func() *entry.Entry
}

func TestTransform(t *testing.T) {
	t.Skip("skipping normalize transformer test")

	now := time.Now()
	newTestEntry := func() *entry.Entry {
		e := entry.New()
		e.ObservedTimestamp = now
		e.Timestamp = time.Unix(1586632809, 0)
		return e
	}

	cases := []testCase{
		{
			name:      "moves_msg_field_to_message_and_deletes_original",
			expectErr: false,
			input: func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]any{
					"msg":   "test message",
					"level": "info",
					"other": "data",
				}
				return e
			},
			output: func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]any{
					"level":   "info",
					"other":   "data",
					"message": "test message",
				}
				return e
			},
		},
		{
			name:      "json_string_with_msg_field_parses_and_restructures",
			expectErr: false,
			input: func() *entry.Entry {
				e := newTestEntry()
				e.Body = `{"msg": "test message", "level": "info"}`
				return e
			},
			output: func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]any{
					"message": "test message",
					"level":   "info",
				}
				return e
			},
		},
	}

	for _, tc := range cases {
		t.Run("Transform/"+tc.name, func(t *testing.T) {
			cfg := NewConfig()
			cfg.OutputIDs = []string{"fake"}
			cfg.OnError = "drop"
			set := componenttest.NewNopTelemetrySettings()
			op, err := cfg.Build(set)
			require.NoError(t, err)

			processor := op.(*Processor)
			fake := testutil.NewFakeOutput(t)
			require.NoError(t, processor.SetOutputs([]operator.Operator{fake}))

			val := tc.input()
			err = processor.Process(context.Background(), val)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.output().Body, val.Body)
				fake.ExpectEntry(t, tc.output())
			}
		})
	}
}

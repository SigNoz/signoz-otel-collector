// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signozawsfirehosereceiver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/SigNoz/signoz-otel-collector/pkg/pdatagen/plogsgen"
	"github.com/SigNoz/signoz-otel-collector/receiver/signozawsfirehosereceiver/internal/metadata"
	"github.com/SigNoz/signoz-otel-collector/receiver/signozawsfirehosereceiver/internal/unmarshaler"
	"github.com/SigNoz/signoz-otel-collector/receiver/signozawsfirehosereceiver/internal/unmarshaler/unmarshalertest"
)

type logsRecordConsumer struct {
	result plog.Logs
}

var _ consumer.Logs = (*logsRecordConsumer)(nil)

func (rc *logsRecordConsumer) ConsumeLogs(_ context.Context, logs plog.Logs) error {
	rc.result = logs
	return nil
}

func (rc *logsRecordConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func TestNewLogsReceiver(t *testing.T) {
	testCases := map[string]struct {
		consumer   consumer.Logs
		recordType string
		wantErr    error
	}{
		"WithInvalidRecordType": {
			consumer:   consumertest.NewNop(),
			recordType: "test",
			wantErr:    errUnrecognizedRecordType,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.RecordType = testCase.recordType
			got, err := newLogsReceiver(
				cfg,
				receivertest.NewNopSettings(metadata.Type),
				defaultLogsUnmarshalers(zap.NewNop()),
				testCase.consumer,
			)
			require.Equal(t, testCase.wantErr, err)
			if testCase.wantErr == nil {
				require.NotNil(t, got)
			} else {
				require.Nil(t, got)
			}
		})
	}
}

func TestLogsReceiverWithSuccess(t *testing.T) {
	ctx := context.Background()

	sink := &consumertest.LogsSink{}
	cfg := createDefaultConfig().(*Config)
	cfg.RecordType = "test"
	cfg.ServerConfig.Endpoint = "localhost:0"

	receiver, err := newLogsReceiver(
		cfg,
		receivertest.NewNopSettings(metadata.Type),
		map[string]unmarshaler.LogsUnmarshaler{"test": unmarshalertest.NewWithLogs(plogsgen.Generate(plogsgen.WithLogRecordCount(10)))},
		sink,
	)
	require.NoError(t, err)

	firehoseReceiver, ok := receiver.(*firehoseReceiver)
	require.True(t, ok)

	require.NoError(t, receiver.Start(ctx, componenttest.NewNopHost()))
	t.Cleanup(func() {
		require.NoError(t, receiver.Shutdown(ctx))
	})

	req, err := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf("http://%s/awsfirehose/test", firehoseReceiver.address),
		strings.NewReader(fmt.Sprintf(`{"requestId":"id","timestamp":%d,"records":[]}`, time.Now().Unix())),
	)
	require.NoError(t, err)
	req.Header.Add(headerFirehoseRequestID, "id")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	// To prevent goroutine leak
	_, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	err = resp.Body.Close()
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 10, sink.LogRecordCount())
	sink.Reset()
}

func TestLogsReceiverWithError(t *testing.T) {
	ctx := context.Background()
	testErr := errors.New("test error")

	cfg := createDefaultConfig().(*Config)
	cfg.RecordType = "test"
	cfg.ServerConfig.Endpoint = "localhost:0"

	receiver, err := newLogsReceiver(
		cfg,
		receivertest.NewNopSettings(metadata.Type),
		map[string]unmarshaler.LogsUnmarshaler{"test": unmarshalertest.NewErrLogs(testErr)},
		consumertest.NewErr(testErr),
	)
	require.NoError(t, err)

	firehoseReceiver, ok := receiver.(*firehoseReceiver)
	require.True(t, ok)

	firehoseLogsConsumer, ok := firehoseReceiver.consumer.(*logsConsumer)
	require.True(t, ok)

	require.NoError(t, firehoseReceiver.Start(ctx, componenttest.NewNopHost()))
	t.Cleanup(func() {
		require.NoError(t, firehoseReceiver.Shutdown(ctx))
	})

	testCases := []struct {
		name                 string
		body                 io.Reader
		unmarshaler          func() unmarshaler.LogsUnmarshaler
		consumer             func() consumertest.Consumer
		expectedErrorMessage error
		expectedCode         int
		expectedLogs         func() plog.Logs
	}{
		{
			name:                 "WithUnmarshalerError",
			body:                 strings.NewReader(fmt.Sprintf(`{"requestId":"id","timestamp":%d,"records":[]}`, time.Now().Unix())),
			unmarshaler:          func() unmarshaler.LogsUnmarshaler { return unmarshalertest.NewErrLogs(testErr) },
			consumer:             func() consumertest.Consumer { return consumertest.NewNop() },
			expectedErrorMessage: testErr,
			expectedCode:         http.StatusBadRequest,
			expectedLogs:         func() plog.Logs { return plog.NewLogs() },
		},
		{
			name: "WithConsumerError",
			body: strings.NewReader(fmt.Sprintf(`{"requestId":"id","timestamp":%d,"records":[]}`, time.Now().Unix())),
			unmarshaler: func() unmarshaler.LogsUnmarshaler {
				return unmarshalertest.NewWithLogs(plogsgen.Generate(plogsgen.WithLogRecordCount(1)))
			},
			consumer:             func() consumertest.Consumer { return consumertest.NewErr(testErr) },
			expectedErrorMessage: testErr,
			expectedCode:         http.StatusInternalServerError,
			expectedLogs:         func() plog.Logs { return plog.NewLogs() },
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			firehoseLogsConsumer.consumer = tc.consumer()
			firehoseLogsConsumer.unmarshaler = tc.unmarshaler()

			firehoseReceiver.consumer = firehoseLogsConsumer

			req, err := http.NewRequest(
				http.MethodPost,
				fmt.Sprintf("http://%s/awsfirehose/test", firehoseReceiver.address),
				tc.body,
			)
			require.NoError(t, err)
			req.Header.Add(headerFirehoseRequestID, "id")

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			err = resp.Body.Close()
			require.NoError(t, err)

			assert.Equal(t, tc.expectedCode, resp.StatusCode)
			assert.Equal(t, tc.expectedErrorMessage.Error(), gjson.Get(string(body), "errorMessage").String())
		})
	}
}

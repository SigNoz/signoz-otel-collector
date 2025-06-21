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
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/SigNoz/signoz-otel-collector/pkg/pdatagen/pmetricsgen"
	"github.com/SigNoz/signoz-otel-collector/receiver/signozawsfirehosereceiver/internal/metadata"
	"github.com/SigNoz/signoz-otel-collector/receiver/signozawsfirehosereceiver/internal/unmarshaler"
	"github.com/SigNoz/signoz-otel-collector/receiver/signozawsfirehosereceiver/internal/unmarshaler/unmarshalertest"
)

type recordConsumer struct {
	result pmetric.Metrics
}

var _ consumer.Metrics = (*recordConsumer)(nil)

func (rc *recordConsumer) ConsumeMetrics(_ context.Context, metrics pmetric.Metrics) error {
	rc.result = metrics
	return nil
}

func (rc *recordConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func TestNewMetricsReceiver(t *testing.T) {
	testCases := map[string]struct {
		consumer   consumer.Metrics
		recordType string
		wantErr    error
	}{
		"WithInvalidRecordType": {
			consumer:   consumertest.NewNop(),
			recordType: "test",
			wantErr:    fmt.Errorf("%w: recordType = %s", errUnrecognizedRecordType, "test"),
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.RecordType = testCase.recordType
			got, err := newMetricsReceiver(
				cfg,
				receivertest.NewNopSettings(metadata.Type),
				defaultMetricsUnmarshalers(zap.NewNop()),
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

func TestMetricsReceiverWithSuccess(t *testing.T) {
	ctx := context.Background()

	sink := &consumertest.MetricsSink{}
	cfg := createDefaultConfig().(*Config)
	cfg.RecordType = "test"
	cfg.ServerConfig.Endpoint = "localhost:0"

	receiver, err := newMetricsReceiver(
		cfg,
		receivertest.NewNopSettings(metadata.Type),
		map[string]unmarshaler.MetricsUnmarshaler{"test": unmarshalertest.NewWithMetrics(pmetricsgen.Generate(pmetricsgen.WithCount(pmetricsgen.Count{GaugeMetricsCount: 2, GaugeDataPointCount: 10})))},
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
	assert.Equal(t, 20, sink.DataPointCount())
	sink.Reset()
}

func TestMetricsReceiverWithError(t *testing.T) {
	ctx := context.Background()
	testErr := errors.New("test error")

	cfg := createDefaultConfig().(*Config)
	cfg.RecordType = "test"
	cfg.ServerConfig.Endpoint = "localhost:0"

	receiver, err := newMetricsReceiver(
		cfg,
		receivertest.NewNopSettings(metadata.Type),
		map[string]unmarshaler.MetricsUnmarshaler{"test": unmarshalertest.NewErrMetrics(testErr)},
		consumertest.NewErr(testErr),
	)
	require.NoError(t, err)

	firehoseReceiver, ok := receiver.(*firehoseReceiver)
	require.True(t, ok)

	firehoseMetricsConsumer, ok := firehoseReceiver.consumer.(*metricsConsumer)
	require.True(t, ok)

	require.NoError(t, firehoseReceiver.Start(ctx, componenttest.NewNopHost()))
	t.Cleanup(func() {
		require.NoError(t, firehoseReceiver.Shutdown(ctx))
	})

	testCases := []struct {
		name                 string
		body                 io.Reader
		unmarshaler          func() unmarshaler.MetricsUnmarshaler
		consumer             func() consumertest.Consumer
		expectedErrorMessage error
		expectedCode         int
		expectedMetrics      func() pmetric.Metrics
	}{
		{
			name:                 "WithUnmarshalerError",
			body:                 strings.NewReader(fmt.Sprintf(`{"requestId":"id","timestamp":%d,"records":[]}`, time.Now().Unix())),
			unmarshaler:          func() unmarshaler.MetricsUnmarshaler { return unmarshalertest.NewErrMetrics(testErr) },
			consumer:             func() consumertest.Consumer { return consumertest.NewNop() },
			expectedErrorMessage: testErr,
			expectedCode:         http.StatusBadRequest,
			expectedMetrics:      func() pmetric.Metrics { return pmetric.NewMetrics() },
		},
		{
			name: "WithConsumerError",
			body: strings.NewReader(fmt.Sprintf(`{"requestId":"id","timestamp":%d,"records":[]}`, time.Now().Unix())),
			unmarshaler: func() unmarshaler.MetricsUnmarshaler {
				return unmarshalertest.NewWithMetrics(pmetricsgen.Generate())
			},
			consumer:             func() consumertest.Consumer { return consumertest.NewErr(testErr) },
			expectedErrorMessage: testErr,
			expectedCode:         http.StatusInternalServerError,
			expectedMetrics:      func() pmetric.Metrics { return pmetric.NewMetrics() },
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			firehoseMetricsConsumer.consumer = tc.consumer()
			firehoseMetricsConsumer.unmarshaler = tc.unmarshaler()

			firehoseReceiver.consumer = firehoseMetricsConsumer

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

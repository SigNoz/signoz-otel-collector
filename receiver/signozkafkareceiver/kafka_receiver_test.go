// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signozkafkareceiver

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/otel/metric/noop"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/SigNoz/signoz-otel-collector/internal/coreinternal/testdata"
	"github.com/SigNoz/signoz-otel-collector/internal/coreinternal/textutils"
	"github.com/SigNoz/signoz-otel-collector/internal/kafka"

	"github.com/SigNoz/signoz-otel-collector/receiver/signozkafkareceiver/internal/metadata"
)

func TestNewTracesReceiver_version_err(t *testing.T) {
	c := Config{
		Encoding:        defaultEncoding,
		ProtocolVersion: "none",
	}
	r, err := newTracesReceiver(c, receivertest.NewNopSettings(metadata.Type), defaultTracesUnmarshalers(), consumertest.NewNop())
	assert.Error(t, err)
	assert.Nil(t, r)
}

func TestNewTracesReceiver_encoding_err(t *testing.T) {
	c := Config{
		Encoding: "foo",
	}
	r, err := newTracesReceiver(c, receivertest.NewNopSettings(metadata.Type), defaultTracesUnmarshalers(), consumertest.NewNop())
	require.Error(t, err)
	assert.Nil(t, r)
	assert.EqualError(t, err, errUnrecognizedEncoding.Error())
}

func TestNewTracesReceiver_err_auth_type(t *testing.T) {
	c := Config{
		ProtocolVersion: "2.0.0",
		Authentication: kafka.Authentication{
			TLS: &configtls.ClientConfig{
				Config: configtls.Config{
					CAFile: "/doesnotexist",
				},
			},
		},
		Encoding: defaultEncoding,
		Metadata: Metadata{
			Full: false,
		},
	}
	r, err := newTracesReceiver(c, receivertest.NewNopSettings(metadata.Type), defaultTracesUnmarshalers(), consumertest.NewNop())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load TLS config")
	assert.Nil(t, r)
}

func TestNewTracesReceiver_initial_offset_err(t *testing.T) {
	c := Config{
		InitialOffset: "foo",
		Encoding:      defaultEncoding,
	}
	r, err := newTracesReceiver(c, receivertest.NewNopSettings(metadata.Type), defaultTracesUnmarshalers(), consumertest.NewNop())
	require.Error(t, err)
	assert.Nil(t, r)
	assert.EqualError(t, err, errInvalidInitialOffset.Error())
}

func TestTracesReceiverStart(t *testing.T) {
	consumerGroup := &testConsumerGroup{}
	c := kafkaTracesConsumer{
		nextConsumer:  consumertest.NewNop(),
		settings:      receivertest.NewNopSettings(metadata.Type),
		consumerGroup: consumerGroup,
	}

	require.NoError(t, c.Start(context.Background(), componenttest.NewNopHost()))

	// Ensure shutdown is called
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, c.Shutdown(ctx))

	// Give time for goroutines to exit
	time.Sleep(100 * time.Millisecond)
}

func TestTracesReceiverStartConsume(t *testing.T) {
	consumerGroup := &testConsumerGroup{}
	c := kafkaTracesConsumer{
		nextConsumer:  consumertest.NewNop(),
		settings:      receivertest.NewNopSettings(metadata.Type),
		consumerGroup: consumerGroup,
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	c.cancelConsumeLoop = cancelFunc

	metrics, err := NewKafkaReceiverMetrics(c.settings.MeterProvider.Meter(metadata.ScopeName))
	require.NoError(t, err)

	handler := &tracesConsumerGroupHandler{
		ready: make(chan bool),
		baseConsumerGroupHandler: baseConsumerGroupHandler{
			metrics: metrics,
		},
	}

	// Start consume loop in goroutine
	done := make(chan error, 1)
	go func() {
		done <- c.consumeLoop(ctx, handler)
	}()

	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)

	// Cancel the context
	cancelFunc()

	// Wait for consume loop to exit
	select {
	case err := <-done:
		assert.EqualError(t, err, context.Canceled.Error())
	case <-time.After(5 * time.Second):
		t.Fatal("consume loop did not exit in time")
	}

	// Shutdown to clean up
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	require.NoError(t, c.Shutdown(shutdownCtx))
}

func TestTracesReceiver_error(t *testing.T) {
	zcore, logObserver := observer.New(zapcore.ErrorLevel)
	logger := zap.New(zcore)
	settings := receivertest.NewNopSettings(metadata.Type)
	settings.Logger = logger

	expectedErr := errors.New("handler error")
	consumerGroup := &testConsumerGroup{err: expectedErr}
	c := kafkaTracesConsumer{
		nextConsumer:  consumertest.NewNop(),
		settings:      settings,
		consumerGroup: consumerGroup,
	}

	require.NoError(t, c.Start(context.Background(), componenttest.NewNopHost()))

	// Wait for error to be logged
	assert.Eventually(t, func() bool {
		return logObserver.FilterField(zap.Error(expectedErr)).Len() > 0
	}, 10*time.Second, time.Millisecond*100)

	// Shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, c.Shutdown(ctx))
}

func TestTracesConsumerGroupHandler(t *testing.T) {
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type)})
	require.NoError(t, err)
	metrics, err := NewKafkaReceiverMetrics(noop.NewMeterProvider().Meter(metadata.ScopeName))
	require.NoError(t, err)
	c := tracesConsumerGroupHandler{
		unmarshaler:  newPdataTracesUnmarshaler(&ptrace.ProtoUnmarshaler{}, defaultEncoding),
		logger:       zap.NewNop(),
		ready:        make(chan bool),
		nextConsumer: consumertest.NewNop(),
		obsrecv:      obsrecv,
		baseConsumerGroupHandler: baseConsumerGroupHandler{
			metrics: metrics,
		},
	}

	testSession := testConsumerGroupSession{ctx: context.Background()}
	require.NoError(t, c.Setup(testSession))
	_, ok := <-c.ready
	assert.False(t, ok)

	require.NoError(t, c.Cleanup(testSession))

	groupClaim := testConsumerGroupClaim{
		messageChan: make(chan *sarama.ConsumerMessage),
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		require.NoError(t, c.ConsumeClaim(testSession, groupClaim))
		wg.Done()
	}()

	groupClaim.messageChan <- &sarama.ConsumerMessage{}
	close(groupClaim.messageChan)
	wg.Wait()
}

func TestTracesConsumerGroupHandlerWithMemoryLimiter(t *testing.T) {
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type)})
	require.NoError(t, err)

	group := &testConsumerGroup{}
	metrics, err := NewKafkaReceiverMetrics(noop.NewMeterProvider().Meter(metadata.ScopeName))
	require.NoError(t, err)
	c := tracesConsumerGroupHandler{
		unmarshaler:  newPdataTracesUnmarshaler(&ptrace.ProtoUnmarshaler{}, defaultEncoding),
		logger:       zap.NewNop(),
		ready:        make(chan bool),
		nextConsumer: consumertest.NewErr(errHighMemoryUsage),
		obsrecv:      obsrecv,
		baseConsumerGroupHandler: baseConsumerGroupHandler{
			group:           group,
			logger:          zap.NewNop(),
			retryInterval:   1 * time.Second,
			pausePartition:  make(chan struct{}),
			resumePartition: make(chan struct{}),
			metrics:         metrics,
		},
	}

	sessionContext, cancel := context.WithCancel(context.Background())
	defer cancel()
	testSession := testConsumerGroupSession{ctx: sessionContext}
	require.NoError(t, c.Setup(testSession))
	_, ok := <-c.ready
	assert.False(t, ok)

	groupClaim := testConsumerGroupClaim{
		messageChan: make(chan *sarama.ConsumerMessage),
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		require.Error(t, c.ConsumeClaim(testSession, groupClaim))
		assert.True(t, group.paused)
		wg.Done()
	}()

	groupClaim.messageChan <- &sarama.ConsumerMessage{}
	close(groupClaim.messageChan)
	cancel()
	wg.Wait()
}

func TestTracesConsumerGroupHandler_session_done(t *testing.T) {
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type)})
	require.NoError(t, err)
	metrics, err := NewKafkaReceiverMetrics(noop.NewMeterProvider().Meter(metadata.ScopeName))
	require.NoError(t, err)
	c := tracesConsumerGroupHandler{
		unmarshaler:  newPdataTracesUnmarshaler(&ptrace.ProtoUnmarshaler{}, defaultEncoding),
		logger:       zap.NewNop(),
		ready:        make(chan bool),
		nextConsumer: consumertest.NewNop(),
		obsrecv:      obsrecv,
		baseConsumerGroupHandler: baseConsumerGroupHandler{
			metrics: metrics,
		},
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	testSession := testConsumerGroupSession{ctx: ctx}
	require.NoError(t, c.Setup(testSession))
	_, ok := <-c.ready
	assert.False(t, ok)

	require.NoError(t, c.Cleanup(testSession))

	groupClaim := testConsumerGroupClaim{
		messageChan: make(chan *sarama.ConsumerMessage),
	}
	defer close(groupClaim.messageChan)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		require.NoError(t, c.ConsumeClaim(testSession, groupClaim))
		wg.Done()
	}()

	groupClaim.messageChan <- &sarama.ConsumerMessage{}
	cancelFunc()
	wg.Wait()
}

func TestTracesConsumerGroupHandler_error_unmarshal(t *testing.T) {
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type)})
	require.NoError(t, err)
	metrics, err := NewKafkaReceiverMetrics(noop.NewMeterProvider().Meter(metadata.ScopeName))
	require.NoError(t, err)
	c := tracesConsumerGroupHandler{
		unmarshaler:  newPdataTracesUnmarshaler(&ptrace.ProtoUnmarshaler{}, defaultEncoding),
		logger:       zap.NewNop(),
		ready:        make(chan bool),
		nextConsumer: consumertest.NewNop(),
		obsrecv:      obsrecv,
		baseConsumerGroupHandler: baseConsumerGroupHandler{
			metrics: metrics,
		},
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	groupClaim := &testConsumerGroupClaim{
		messageChan: make(chan *sarama.ConsumerMessage),
	}
	go func() {
		err := c.ConsumeClaim(testConsumerGroupSession{ctx: context.Background()}, groupClaim)
		require.Error(t, err)
		wg.Done()
	}()
	groupClaim.messageChan <- &sarama.ConsumerMessage{Value: []byte("!@#")}
	close(groupClaim.messageChan)
	wg.Wait()
}

func TestTracesConsumerGroupHandler_error_nextConsumer(t *testing.T) {
	consumerError := errors.New("failed to consume")
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type)})
	require.NoError(t, err)
	metrics, err := NewKafkaReceiverMetrics(noop.NewMeterProvider().Meter(metadata.ScopeName))
	require.NoError(t, err)
	c := tracesConsumerGroupHandler{
		unmarshaler:  newPdataTracesUnmarshaler(&ptrace.ProtoUnmarshaler{}, defaultEncoding),
		logger:       zap.NewNop(),
		ready:        make(chan bool),
		nextConsumer: consumertest.NewErr(consumerError),
		obsrecv:      obsrecv,
		baseConsumerGroupHandler: baseConsumerGroupHandler{
			metrics: metrics,
		},
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	groupClaim := &testConsumerGroupClaim{
		messageChan: make(chan *sarama.ConsumerMessage),
	}
	go func() {
		e := c.ConsumeClaim(testConsumerGroupSession{ctx: context.Background()}, groupClaim)
		assert.EqualError(t, e, consumerError.Error())
		wg.Done()
	}()

	td := ptrace.NewTraces()
	td.ResourceSpans().AppendEmpty()
	unmarshaler := &ptrace.ProtoMarshaler{}
	bts, err := unmarshaler.MarshalTraces(td)
	require.NoError(t, err)
	groupClaim.messageChan <- &sarama.ConsumerMessage{Value: bts}
	close(groupClaim.messageChan)
	wg.Wait()
}

func TestNewMetricsReceiver_version_err(t *testing.T) {
	c := Config{
		Encoding:        defaultEncoding,
		ProtocolVersion: "none",
	}
	r, err := newMetricsReceiver(c, receivertest.NewNopSettings(metadata.Type), defaultMetricsUnmarshalers(), consumertest.NewNop())
	assert.Error(t, err)
	assert.Nil(t, r)
}

func TestNewMetricsReceiver_encoding_err(t *testing.T) {
	c := Config{
		Encoding: "foo",
	}
	r, err := newMetricsReceiver(c, receivertest.NewNopSettings(metadata.Type), defaultMetricsUnmarshalers(), consumertest.NewNop())
	require.Error(t, err)
	assert.Nil(t, r)
	assert.EqualError(t, err, errUnrecognizedEncoding.Error())
}

func TestNewMetricsExporter_err_auth_type(t *testing.T) {
	c := Config{
		ProtocolVersion: "2.0.0",
		Authentication: kafka.Authentication{
			TLS: &configtls.ClientConfig{
				Config: configtls.Config{
					CAFile: "/doesnotexist",
				},
			},
		},
		Encoding: defaultEncoding,
		Metadata: Metadata{
			Full: false,
		},
	}
	r, err := newMetricsReceiver(c, receivertest.NewNopSettings(metadata.Type), defaultMetricsUnmarshalers(), consumertest.NewNop())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load TLS config")
	assert.Nil(t, r)
}

func TestNewMetricsReceiver_initial_offset_err(t *testing.T) {
	c := Config{
		InitialOffset: "foo",
		Encoding:      defaultEncoding,
	}
	r, err := newMetricsReceiver(c, receivertest.NewNopSettings(metadata.Type), defaultMetricsUnmarshalers(), consumertest.NewNop())
	require.Error(t, err)
	assert.Nil(t, r)
	assert.EqualError(t, err, errInvalidInitialOffset.Error())
}

func TestMetricsReceiverStart(t *testing.T) {
	consumerGroup := &testConsumerGroup{}
	c := kafkaMetricsConsumer{
		nextConsumer:  consumertest.NewNop(),
		settings:      receivertest.NewNopSettings(metadata.Type),
		consumerGroup: consumerGroup,
	}

	require.NoError(t, c.Start(context.Background(), componenttest.NewNopHost()))

	// Ensure shutdown is called
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, c.Shutdown(ctx))

	// Give time for goroutines to exit
	time.Sleep(100 * time.Millisecond)
}

func TestMetricsReceiverStartConsume(t *testing.T) {
	consumerGroup := &testConsumerGroup{}
	c := kafkaMetricsConsumer{
		nextConsumer:  consumertest.NewNop(),
		settings:      receivertest.NewNopSettings(metadata.Type),
		consumerGroup: consumerGroup,
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	c.cancelConsumeLoop = cancelFunc

	metrics, err := NewKafkaReceiverMetrics(c.settings.MeterProvider.Meter(metadata.ScopeName))
	require.NoError(t, err)

	handler := &metricsConsumerGroupHandler{
		ready: make(chan bool),
		baseConsumerGroupHandler: baseConsumerGroupHandler{
			metrics: metrics,
		},
	}

	// Start consume loop in goroutine
	done := make(chan error, 1)
	go func() {
		done <- c.consumeLoop(ctx, handler)
	}()

	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)

	// Cancel the context
	cancelFunc()

	// Wait for consume loop to exit
	select {
	case err := <-done:
		assert.EqualError(t, err, context.Canceled.Error())
	case <-time.After(5 * time.Second):
		t.Fatal("consume loop did not exit in time")
	}

	// Shutdown to clean up
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	require.NoError(t, c.Shutdown(shutdownCtx))
}

func TestMetricsReceiver_error(t *testing.T) {
	zcore, logObserver := observer.New(zapcore.ErrorLevel)
	logger := zap.New(zcore)
	settings := receivertest.NewNopSettings(metadata.Type)
	settings.Logger = logger

	expectedErr := errors.New("handler error")
	consumerGroup := &testConsumerGroup{err: expectedErr}
	c := kafkaMetricsConsumer{
		nextConsumer:  consumertest.NewNop(),
		settings:      settings,
		consumerGroup: consumerGroup,
	}

	require.NoError(t, c.Start(context.Background(), componenttest.NewNopHost()))

	// Wait for error to be logged
	assert.Eventually(t, func() bool {
		return logObserver.FilterField(zap.Error(expectedErr)).Len() > 0
	}, 10*time.Second, time.Millisecond*100)

	// Shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, c.Shutdown(ctx))
}

func TestMetricsConsumerGroupHandler(t *testing.T) {
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type)})
	require.NoError(t, err)
	metrics, err := NewKafkaReceiverMetrics(noop.NewMeterProvider().Meter(metadata.ScopeName))
	require.NoError(t, err)
	c := metricsConsumerGroupHandler{
		unmarshaler:  newPdataMetricsUnmarshaler(&pmetric.ProtoUnmarshaler{}, defaultEncoding),
		logger:       zap.NewNop(),
		ready:        make(chan bool),
		nextConsumer: consumertest.NewNop(),
		obsrecv:      obsrecv,
		baseConsumerGroupHandler: baseConsumerGroupHandler{
			metrics: metrics,
		},
	}

	testSession := testConsumerGroupSession{ctx: context.Background()}
	require.NoError(t, c.Setup(testSession))
	_, ok := <-c.ready
	assert.False(t, ok)

	require.NoError(t, c.Cleanup(testSession))

	groupClaim := testConsumerGroupClaim{
		messageChan: make(chan *sarama.ConsumerMessage),
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		require.NoError(t, c.ConsumeClaim(testSession, groupClaim))
		wg.Done()
	}()

	groupClaim.messageChan <- &sarama.ConsumerMessage{}
	close(groupClaim.messageChan)
	wg.Wait()
}

func TestMetricsConsumerGroupHandlerWithMemoryLimiter(t *testing.T) {
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type)})
	require.NoError(t, err)

	group := &testConsumerGroup{}
	metrics, err := NewKafkaReceiverMetrics(noop.NewMeterProvider().Meter(metadata.ScopeName))
	require.NoError(t, err)
	c := metricsConsumerGroupHandler{
		unmarshaler:  newPdataMetricsUnmarshaler(&pmetric.ProtoUnmarshaler{}, defaultEncoding),
		logger:       zap.NewNop(),
		ready:        make(chan bool),
		nextConsumer: consumertest.NewErr(errHighMemoryUsage),
		obsrecv:      obsrecv,
		baseConsumerGroupHandler: baseConsumerGroupHandler{
			group:           group,
			logger:          zap.NewNop(),
			retryInterval:   1 * time.Second,
			pausePartition:  make(chan struct{}),
			resumePartition: make(chan struct{}),
			metrics:         metrics,
		},
	}

	sessionContext, cancel := context.WithCancel(context.Background())
	defer cancel()
	testSession := testConsumerGroupSession{ctx: sessionContext}
	require.NoError(t, c.Setup(testSession))
	_, ok := <-c.ready
	assert.False(t, ok)

	groupClaim := testConsumerGroupClaim{
		messageChan: make(chan *sarama.ConsumerMessage),
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		require.Error(t, c.ConsumeClaim(testSession, groupClaim))
		assert.True(t, group.paused)
		wg.Done()
	}()

	groupClaim.messageChan <- &sarama.ConsumerMessage{}
	close(groupClaim.messageChan)
	cancel()
	wg.Wait()
}

func TestMetricsConsumerGroupHandler_session_done(t *testing.T) {
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type)})
	require.NoError(t, err)
	metrics, err := NewKafkaReceiverMetrics(noop.NewMeterProvider().Meter(metadata.ScopeName))
	require.NoError(t, err)
	c := metricsConsumerGroupHandler{
		unmarshaler:  newPdataMetricsUnmarshaler(&pmetric.ProtoUnmarshaler{}, defaultEncoding),
		logger:       zap.NewNop(),
		ready:        make(chan bool),
		nextConsumer: consumertest.NewNop(),
		obsrecv:      obsrecv,
		baseConsumerGroupHandler: baseConsumerGroupHandler{
			metrics: metrics,
		},
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	testSession := testConsumerGroupSession{ctx: ctx}
	require.NoError(t, c.Setup(testSession))
	_, ok := <-c.ready
	assert.False(t, ok)

	require.NoError(t, c.Cleanup(testSession))

	groupClaim := testConsumerGroupClaim{
		messageChan: make(chan *sarama.ConsumerMessage),
	}
	defer close(groupClaim.messageChan)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		require.NoError(t, c.ConsumeClaim(testSession, groupClaim))
		wg.Done()
	}()

	groupClaim.messageChan <- &sarama.ConsumerMessage{}
	cancelFunc()
	wg.Wait()
}

func TestMetricsConsumerGroupHandler_error_unmarshal(t *testing.T) {
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type)})
	require.NoError(t, err)
	metrics, err := NewKafkaReceiverMetrics(noop.NewMeterProvider().Meter(metadata.ScopeName))
	require.NoError(t, err)
	c := metricsConsumerGroupHandler{
		unmarshaler:  newPdataMetricsUnmarshaler(&pmetric.ProtoUnmarshaler{}, defaultEncoding),
		logger:       zap.NewNop(),
		ready:        make(chan bool),
		nextConsumer: consumertest.NewNop(),
		obsrecv:      obsrecv,
		baseConsumerGroupHandler: baseConsumerGroupHandler{
			metrics: metrics,
		},
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	groupClaim := &testConsumerGroupClaim{
		messageChan: make(chan *sarama.ConsumerMessage),
	}
	go func() {
		err := c.ConsumeClaim(testConsumerGroupSession{ctx: context.Background()}, groupClaim)
		require.Error(t, err)
		wg.Done()
	}()
	groupClaim.messageChan <- &sarama.ConsumerMessage{Value: []byte("!@#")}
	close(groupClaim.messageChan)
	wg.Wait()
}

func TestMetricsConsumerGroupHandler_error_nextConsumer(t *testing.T) {
	consumerError := errors.New("failed to consume")
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type)})
	require.NoError(t, err)
	metrics, err := NewKafkaReceiverMetrics(noop.NewMeterProvider().Meter(metadata.ScopeName))
	require.NoError(t, err)
	c := metricsConsumerGroupHandler{
		unmarshaler:  newPdataMetricsUnmarshaler(&pmetric.ProtoUnmarshaler{}, defaultEncoding),
		logger:       zap.NewNop(),
		ready:        make(chan bool),
		nextConsumer: consumertest.NewErr(consumerError),
		obsrecv:      obsrecv,
		baseConsumerGroupHandler: baseConsumerGroupHandler{
			metrics: metrics,
		},
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	groupClaim := &testConsumerGroupClaim{
		messageChan: make(chan *sarama.ConsumerMessage),
	}
	go func() {
		e := c.ConsumeClaim(testConsumerGroupSession{ctx: context.Background()}, groupClaim)
		assert.EqualError(t, e, consumerError.Error())
		wg.Done()
	}()

	ld := testdata.GenerateMetricsOneMetric()
	unmarshaler := &pmetric.ProtoMarshaler{}
	bts, err := unmarshaler.MarshalMetrics(ld)
	require.NoError(t, err)
	groupClaim.messageChan <- &sarama.ConsumerMessage{Value: bts}
	close(groupClaim.messageChan)
	wg.Wait()
}

func TestNewLogsReceiver_version_err(t *testing.T) {
	c := Config{
		Encoding:        defaultEncoding,
		ProtocolVersion: "none",
	}
	r, err := newLogsReceiver(c, receivertest.NewNopSettings(metadata.Type), defaultLogsUnmarshalers(), consumertest.NewNop())
	assert.Error(t, err)
	assert.Nil(t, r)
}

func TestNewLogsReceiver_encoding_err(t *testing.T) {
	c := Config{
		Encoding: "foo",
	}
	r, err := newLogsReceiver(c, receivertest.NewNopSettings(metadata.Type), defaultLogsUnmarshalers(), consumertest.NewNop())
	require.Error(t, err)
	assert.Nil(t, r)
	assert.EqualError(t, err, errUnrecognizedEncoding.Error())
}

func TestNewLogsExporter_err_auth_type(t *testing.T) {
	c := Config{
		ProtocolVersion: "2.0.0",
		Authentication: kafka.Authentication{
			TLS: &configtls.ClientConfig{
				Config: configtls.Config{
					CAFile: "/doesnotexist",
				},
			},
		},
		Encoding: defaultEncoding,
		Metadata: Metadata{
			Full: false,
		},
	}
	r, err := newLogsReceiver(c, receivertest.NewNopSettings(metadata.Type), defaultLogsUnmarshalers(), consumertest.NewNop())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load TLS config")
	assert.Nil(t, r)
}

func TestNewLogsReceiver_initial_offset_err(t *testing.T) {
	c := Config{
		InitialOffset: "foo",
		Encoding:      defaultEncoding,
	}
	r, err := newLogsReceiver(c, receivertest.NewNopSettings(metadata.Type), defaultLogsUnmarshalers(), consumertest.NewNop())
	require.Error(t, err)
	assert.Nil(t, r)
	assert.EqualError(t, err, errInvalidInitialOffset.Error())
}

func TestLogsReceiverStart(t *testing.T) {
	consumerGroup := &testConsumerGroup{}
	c := kafkaLogsConsumer{
		nextConsumer:  consumertest.NewNop(),
		settings:      receivertest.NewNopSettings(metadata.Type),
		consumerGroup: consumerGroup,
	}

	require.NoError(t, c.Start(context.Background(), componenttest.NewNopHost()))

	// Ensure shutdown is called
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, c.Shutdown(ctx))

	// Give time for goroutines to exit
	time.Sleep(100 * time.Millisecond)
}

func TestLogsReceiverStartConsume(t *testing.T) {
	consumerGroup := &testConsumerGroup{}
	c := kafkaLogsConsumer{
		nextConsumer:  consumertest.NewNop(),
		settings:      receivertest.NewNopSettings(metadata.Type),
		consumerGroup: consumerGroup,
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	c.cancelConsumeLoop = cancelFunc

	metrics, err := NewKafkaReceiverMetrics(c.settings.MeterProvider.Meter(metadata.ScopeName))
	require.NoError(t, err)

	handler := &logsConsumerGroupHandler{
		ready: make(chan bool),
		baseConsumerGroupHandler: baseConsumerGroupHandler{
			metrics: metrics,
		},
	}

	// Start consume loop in goroutine
	done := make(chan error, 1)
	go func() {
		done <- c.consumeLoop(ctx, handler)
	}()

	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)

	// Cancel the context
	cancelFunc()

	// Wait for consume loop to exit
	select {
	case err := <-done:
		assert.EqualError(t, err, context.Canceled.Error())
	case <-time.After(5 * time.Second):
		t.Fatal("consume loop did not exit in time")
	}

	// Shutdown to clean up
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	require.NoError(t, c.Shutdown(shutdownCtx))
}

func TestLogsReceiver_error(t *testing.T) {
	zcore, logObserver := observer.New(zapcore.ErrorLevel)
	logger := zap.New(zcore)
	settings := receivertest.NewNopSettings(metadata.Type)
	settings.Logger = logger

	expectedErr := errors.New("handler error")
	consumerGroup := &testConsumerGroup{err: expectedErr}
	c := kafkaLogsConsumer{
		nextConsumer:  consumertest.NewNop(),
		settings:      settings,
		consumerGroup: consumerGroup,
	}

	require.NoError(t, c.Start(context.Background(), componenttest.NewNopHost()))

	// Wait for error to be logged
	assert.Eventually(t, func() bool {
		return logObserver.FilterField(zap.Error(expectedErr)).Len() > 0
	}, 10*time.Second, time.Millisecond*100)

	// Shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, c.Shutdown(ctx))
}

func TestLogsConsumerGroupHandler(t *testing.T) {
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type)})
	require.NoError(t, err)
	metrics, err := NewKafkaReceiverMetrics(noop.NewMeterProvider().Meter(metadata.ScopeName))
	require.NoError(t, err)
	c := logsConsumerGroupHandler{
		unmarshaler:  NewPdataLogsUnmarshaler(&plog.ProtoUnmarshaler{}, defaultEncoding),
		logger:       zap.NewNop(),
		ready:        make(chan bool),
		nextConsumer: consumertest.NewNop(),
		obsrecv:      obsrecv,
		baseConsumerGroupHandler: baseConsumerGroupHandler{
			metrics: metrics,
		},
	}

	testSession := testConsumerGroupSession{ctx: context.Background()}
	require.NoError(t, c.Setup(testSession))
	_, ok := <-c.ready
	assert.False(t, ok)

	require.NoError(t, c.Cleanup(testSession))

	groupClaim := testConsumerGroupClaim{
		messageChan: make(chan *sarama.ConsumerMessage),
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		require.NoError(t, c.ConsumeClaim(testSession, groupClaim))
		wg.Done()
	}()

	groupClaim.messageChan <- &sarama.ConsumerMessage{}
	close(groupClaim.messageChan)
	wg.Wait()
}

func TestLogsConsumerGroupHandlerWithMemoryLimiter(t *testing.T) {
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type)})
	require.NoError(t, err)

	group := &testConsumerGroup{}
	metrics, err := NewKafkaReceiverMetrics(noop.NewMeterProvider().Meter(metadata.ScopeName))
	require.NoError(t, err)
	c := logsConsumerGroupHandler{
		unmarshaler:  NewPdataLogsUnmarshaler(&plog.ProtoUnmarshaler{}, defaultEncoding),
		logger:       zap.NewNop(),
		ready:        make(chan bool),
		nextConsumer: consumertest.NewErr(errHighMemoryUsage),
		obsrecv:      obsrecv,
		baseConsumerGroupHandler: baseConsumerGroupHandler{
			group:           group,
			logger:          zap.NewNop(),
			retryInterval:   1 * time.Second,
			pausePartition:  make(chan struct{}),
			resumePartition: make(chan struct{}),
			metrics:         metrics,
		},
	}

	sessionContext, cancel := context.WithCancel(context.Background())
	defer cancel()
	testSession := testConsumerGroupSession{ctx: sessionContext}
	require.NoError(t, c.Setup(testSession))
	_, ok := <-c.ready
	assert.False(t, ok)

	groupClaim := testConsumerGroupClaim{
		messageChan: make(chan *sarama.ConsumerMessage),
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		require.Error(t, c.ConsumeClaim(testSession, groupClaim))
		assert.True(t, group.paused)
		wg.Done()
	}()

	groupClaim.messageChan <- &sarama.ConsumerMessage{}
	close(groupClaim.messageChan)
	cancel()
	wg.Wait()
}

func TestLogsConsumerGroupHandler_session_done(t *testing.T) {
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type)})
	require.NoError(t, err)
	metrics, err := NewKafkaReceiverMetrics(noop.NewMeterProvider().Meter(metadata.ScopeName))
	require.NoError(t, err)
	c := logsConsumerGroupHandler{
		unmarshaler:  NewPdataLogsUnmarshaler(&plog.ProtoUnmarshaler{}, defaultEncoding),
		logger:       zap.NewNop(),
		ready:        make(chan bool),
		nextConsumer: consumertest.NewNop(),
		obsrecv:      obsrecv,
		baseConsumerGroupHandler: baseConsumerGroupHandler{
			metrics: metrics,
		},
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	testSession := testConsumerGroupSession{ctx: ctx}
	require.NoError(t, c.Setup(testSession))
	_, ok := <-c.ready
	assert.False(t, ok)

	require.NoError(t, c.Cleanup(testSession))

	groupClaim := testConsumerGroupClaim{
		messageChan: make(chan *sarama.ConsumerMessage),
	}
	defer close(groupClaim.messageChan)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		require.NoError(t, c.ConsumeClaim(testSession, groupClaim))
		wg.Done()
	}()

	groupClaim.messageChan <- &sarama.ConsumerMessage{}
	cancelFunc()
	wg.Wait()
}

func TestLogsConsumerGroupHandler_error_unmarshal(t *testing.T) {
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type)})
	require.NoError(t, err)
	metrics, err := NewKafkaReceiverMetrics(noop.NewMeterProvider().Meter(metadata.ScopeName))
	require.NoError(t, err)
	c := logsConsumerGroupHandler{
		unmarshaler:  NewPdataLogsUnmarshaler(&plog.ProtoUnmarshaler{}, defaultEncoding),
		logger:       zap.NewNop(),
		ready:        make(chan bool),
		nextConsumer: consumertest.NewNop(),
		obsrecv:      obsrecv,
		baseConsumerGroupHandler: baseConsumerGroupHandler{
			metrics: metrics,
		},
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	groupClaim := &testConsumerGroupClaim{
		messageChan: make(chan *sarama.ConsumerMessage),
	}
	go func() {
		err := c.ConsumeClaim(testConsumerGroupSession{ctx: context.Background()}, groupClaim)
		require.Error(t, err)
		wg.Done()
	}()
	groupClaim.messageChan <- &sarama.ConsumerMessage{Value: []byte("!@#")}
	close(groupClaim.messageChan)
	wg.Wait()
}

func TestLogsConsumerGroupHandler_error_nextConsumer(t *testing.T) {
	consumerError := errors.New("failed to consume")
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type)})
	require.NoError(t, err)
	metrics, err := NewKafkaReceiverMetrics(noop.NewMeterProvider().Meter(metadata.ScopeName))
	require.NoError(t, err)
	c := logsConsumerGroupHandler{
		unmarshaler:  NewPdataLogsUnmarshaler(&plog.ProtoUnmarshaler{}, defaultEncoding),
		logger:       zap.NewNop(),
		ready:        make(chan bool),
		nextConsumer: consumertest.NewErr(consumerError),
		obsrecv:      obsrecv,
		baseConsumerGroupHandler: baseConsumerGroupHandler{
			metrics: metrics,
		},
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	groupClaim := &testConsumerGroupClaim{
		messageChan: make(chan *sarama.ConsumerMessage),
	}
	go func() {
		e := c.ConsumeClaim(testConsumerGroupSession{ctx: context.Background()}, groupClaim)
		assert.EqualError(t, e, consumerError.Error())
		wg.Done()
	}()

	ld := testdata.GenerateLogsOneLogRecord()
	unmarshaler := &plog.ProtoMarshaler{}
	bts, err := unmarshaler.MarshalLogs(ld)
	require.NoError(t, err)
	groupClaim.messageChan <- &sarama.ConsumerMessage{Value: bts}
	close(groupClaim.messageChan)
	wg.Wait()
}

// Test unmarshaler for different charsets and encodings.
func TestLogsConsumerGroupHandler_unmarshal_text(t *testing.T) {
	tests := []struct {
		name string
		text string
		enc  string
	}{
		{
			name: "unmarshal test for Englist (ASCII characters) with text_utf8",
			text: "ASCII characters test",
			enc:  "utf8",
		},
		{
			name: "unmarshal test for unicode with text_utf8",
			text: "UTF8 测试 測試 テスト 테스트 ☺️",
			enc:  "utf8",
		},
		{
			name: "unmarshal test for Simplified Chinese with text_gbk",
			text: "GBK 简体中文解码测试",
			enc:  "gbk",
		},
		{
			name: "unmarshal test for Japanese with text_shift_jis",
			text: "Shift_JIS 日本のデコードテスト",
			enc:  "shift_jis",
		},
		{
			name: "unmarshal test for Korean with text_euc-kr",
			text: "EUC-KR 한국 디코딩 테스트",
			enc:  "euc-kr",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type)})
			require.NoError(t, err)
			unmarshaler := newTextLogsUnmarshaler()
			unmarshaler, err = unmarshaler.WithEnc(test.enc)
			require.NoError(t, err)
			metrics, err := NewKafkaReceiverMetrics(noop.NewMeterProvider().Meter(metadata.ScopeName))
			require.NoError(t, err)
			sink := &consumertest.LogsSink{}
			c := logsConsumerGroupHandler{
				unmarshaler:  unmarshaler,
				logger:       zap.NewNop(),
				ready:        make(chan bool),
				nextConsumer: sink,
				obsrecv:      obsrecv,
				baseConsumerGroupHandler: baseConsumerGroupHandler{
					metrics: metrics,
				},
			}

			wg := sync.WaitGroup{}
			wg.Add(1)
			groupClaim := &testConsumerGroupClaim{
				messageChan: make(chan *sarama.ConsumerMessage),
			}
			go func() {
				err = c.ConsumeClaim(testConsumerGroupSession{ctx: context.Background()}, groupClaim)
				assert.NoError(t, err)
				wg.Done()
			}()
			encCfg := textutils.NewEncodingConfig()
			encCfg.Encoding = test.enc
			enc, err := encCfg.Build()
			require.NoError(t, err)
			encoder := enc.Encoding.NewEncoder()
			encoded, err := encoder.Bytes([]byte(test.text))
			require.NoError(t, err)
			t1 := time.Now()
			groupClaim.messageChan <- &sarama.ConsumerMessage{Value: encoded}
			close(groupClaim.messageChan)
			wg.Wait()
			require.Equal(t, sink.LogRecordCount(), 1)
			log := sink.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
			assert.Equal(t, log.Body().Str(), test.text)
			assert.LessOrEqual(t, t1, log.ObservedTimestamp().AsTime())
			assert.LessOrEqual(t, log.ObservedTimestamp().AsTime(), time.Now())
		})
	}
}

func TestGetLogsUnmarshaler_encoding_text(t *testing.T) {
	tests := []struct {
		name     string
		encoding string
	}{
		{
			name:     "default text encoding",
			encoding: "text",
		},
		{
			name:     "utf-8 text encoding",
			encoding: "text_utf-8",
		},
		{
			name:     "gbk text encoding",
			encoding: "text_gbk",
		},
		{
			name:     "shift_jis text encoding, which contains an underline",
			encoding: "text_shift_jis",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := getLogsUnmarshaler(test.encoding, defaultLogsUnmarshalers())
			assert.NoError(t, err)
		})
	}
}

func TestGetLogsUnmarshaler_encoding_text_error(t *testing.T) {
	tests := []struct {
		name     string
		encoding string
	}{
		{
			name:     "text encoding has typo",
			encoding: "text_uft-8",
		},
		{
			name:     "text encoding is a random string",
			encoding: "text_vnbqgoba156",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := getLogsUnmarshaler(test.encoding, defaultLogsUnmarshalers())
			assert.ErrorContains(t, err, fmt.Sprintf("unsupported encoding '%v'", test.encoding[5:]))
		})
	}
}

func TestCreateLogsReceiver_encoding_text_error(t *testing.T) {
	cfg := Config{
		Encoding: "text_uft-8",
	}
	_, err := newLogsReceiver(cfg, receivertest.NewNopSettings(metadata.Type), defaultLogsUnmarshalers(), consumertest.NewNop())
	// encoding error comes first
	assert.Error(t, err, "unsupported encoding")
}

func TestToSaramaInitialOffset_earliest(t *testing.T) {
	saramaInitialOffset, err := toSaramaInitialOffset(offsetEarliest)

	require.NoError(t, err)
	assert.Equal(t, sarama.OffsetOldest, saramaInitialOffset)
}

func TestToSaramaInitialOffset_latest(t *testing.T) {
	saramaInitialOffset, err := toSaramaInitialOffset(offsetLatest)

	require.NoError(t, err)
	assert.Equal(t, sarama.OffsetNewest, saramaInitialOffset)
}

func TestToSaramaInitialOffset_default(t *testing.T) {
	saramaInitialOffset, err := toSaramaInitialOffset("")

	require.NoError(t, err)
	assert.Equal(t, sarama.OffsetNewest, saramaInitialOffset)
}

func TestToSaramaInitialOffset_invalid(t *testing.T) {
	_, err := toSaramaInitialOffset("other")

	assert.Equal(t, err, errInvalidInitialOffset)
}

type testConsumerGroupClaim struct {
	messageChan chan *sarama.ConsumerMessage
}

var _ sarama.ConsumerGroupClaim = (*testConsumerGroupClaim)(nil)

const (
	testTopic               = "otlp_spans"
	testPartition           = 5
	testInitialOffset       = 6
	testHighWatermarkOffset = 4
)

func (t testConsumerGroupClaim) Topic() string {
	return testTopic
}

func (t testConsumerGroupClaim) Partition() int32 {
	return testPartition
}

func (t testConsumerGroupClaim) InitialOffset() int64 {
	return testInitialOffset
}

func (t testConsumerGroupClaim) HighWaterMarkOffset() int64 {
	return testHighWatermarkOffset
}

func (t testConsumerGroupClaim) Messages() <-chan *sarama.ConsumerMessage {
	return t.messageChan
}

type testConsumerGroupSession struct {
	ctx context.Context
}

func (t testConsumerGroupSession) Commit() {
}

var _ sarama.ConsumerGroupSession = (*testConsumerGroupSession)(nil)

func (t testConsumerGroupSession) Claims() map[string][]int32 {
	panic("implement me")
}

func (t testConsumerGroupSession) MemberID() string {
	panic("implement me")
}

func (t testConsumerGroupSession) GenerationID() int32 {
	panic("implement me")
}

func (t testConsumerGroupSession) MarkOffset(string, int32, int64, string) {
}

func (t testConsumerGroupSession) ResetOffset(string, int32, int64, string) {
	panic("implement me")
}

func (t testConsumerGroupSession) MarkMessage(*sarama.ConsumerMessage, string) {}

func (t testConsumerGroupSession) Context() context.Context {
	return t.ctx
}

// Updated testConsumerGroup with proper cleanup handling
type testConsumerGroup struct {
	once   sync.Once
	err    error
	paused bool
	closed bool
	mu     sync.Mutex

	// Channel to signal when Consume should exit
	consumeDone chan struct{}
}

var _ sarama.ConsumerGroup = (*testConsumerGroup)(nil)

func (t *testConsumerGroup) Consume(ctx context.Context, _ []string, handler sarama.ConsumerGroupHandler) error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return sarama.ErrClosedConsumerGroup
	}
	if t.consumeDone == nil {
		t.consumeDone = make(chan struct{})
	}
	consumeDone := t.consumeDone
	t.mu.Unlock()

	// Setup handler once
	t.once.Do(func() {
		_ = handler.Setup(testConsumerGroupSession{ctx: ctx})
	})

	// If there's an error to return, return it immediately
	if t.err != nil {
		return t.err
	}

	// Block until context is cancelled or Close is called
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-consumeDone:
		return nil
	}
}

func (t *testConsumerGroup) Errors() <-chan error {
	return make(chan error)
}

func (t *testConsumerGroup) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}

	t.closed = true
	if t.consumeDone != nil {
		close(t.consumeDone)
	}
	return nil
}

func (t *testConsumerGroup) Pause(_ map[string][]int32) {
	panic("implement me")
}

func (t *testConsumerGroup) PauseAll() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.paused = true
}

func (t *testConsumerGroup) Resume(_ map[string][]int32) {
	panic("implement me")
}

func (t *testConsumerGroup) ResumeAll() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.paused = false
}

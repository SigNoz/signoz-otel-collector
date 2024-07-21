// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signozkafkareceiver // import "github.com/SigNoz/signoz-otel-collector/receiver/signozkafkareceiver"

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"

	"github.com/SigNoz/signoz-otel-collector/internal/kafka"
)

const (
	transport = "kafka"
)

var errUnrecognizedEncoding = fmt.Errorf("unrecognized encoding")
var errInvalidInitialOffset = fmt.Errorf("invalid initial offset")

// kafkaTracesConsumer uses sarama to consume and handle messages from kafka.
type kafkaTracesConsumer struct {
	consumerGroup     sarama.ConsumerGroup
	nextConsumer      consumer.Traces
	topics            []string
	cancelConsumeLoop context.CancelFunc
	unmarshaler       TracesUnmarshaler

	settings receiver.CreateSettings

	autocommitEnabled bool

	batchSize    int
	batchTimeout time.Duration
}

// kafkaMetricsConsumer uses sarama to consume and handle messages from kafka.
type kafkaMetricsConsumer struct {
	consumerGroup     sarama.ConsumerGroup
	nextConsumer      consumer.Metrics
	topics            []string
	cancelConsumeLoop context.CancelFunc
	unmarshaler       MetricsUnmarshaler

	settings receiver.CreateSettings

	autocommitEnabled bool

	batchSize    int
	batchTimeout time.Duration
}

// kafkaLogsConsumer uses sarama to consume and handle messages from kafka.
type kafkaLogsConsumer struct {
	consumerGroup     sarama.ConsumerGroup
	nextConsumer      consumer.Logs
	topics            []string
	cancelConsumeLoop context.CancelFunc
	unmarshaler       LogsUnmarshaler

	settings receiver.CreateSettings

	autocommitEnabled bool

	batchSize    int
	batchTimeout time.Duration
}

var _ receiver.Traces = (*kafkaTracesConsumer)(nil)
var _ receiver.Metrics = (*kafkaMetricsConsumer)(nil)
var _ receiver.Logs = (*kafkaLogsConsumer)(nil)

func newTracesReceiver(config Config, set receiver.CreateSettings, unmarshalers map[string]TracesUnmarshaler, nextConsumer consumer.Traces) (*kafkaTracesConsumer, error) {
	unmarshaler := unmarshalers[config.Encoding]
	if unmarshaler == nil {
		return nil, errUnrecognizedEncoding
	}

	// set sarama library's logger to get detailed logs from the library
	sarama.Logger = zap.NewStdLog(set.Logger)

	c := sarama.NewConfig()
	c = setSaramaConsumerConfig(c, &config.SaramaConsumerConfig)
	c.ClientID = config.ClientID
	c.Metadata.Full = config.Metadata.Full
	c.Metadata.Retry.Max = config.Metadata.Retry.Max
	c.Metadata.Retry.Backoff = config.Metadata.Retry.Backoff
	c.Consumer.Offsets.AutoCommit.Enable = config.AutoCommit.Enable
	c.Consumer.Offsets.AutoCommit.Interval = config.AutoCommit.Interval

	if initialOffset, err := toSaramaInitialOffset(config.InitialOffset); err == nil {
		c.Consumer.Offsets.Initial = initialOffset
	} else {
		return nil, err
	}

	if config.ProtocolVersion != "" {
		version, err := sarama.ParseKafkaVersion(config.ProtocolVersion)
		if err != nil {
			return nil, err
		}
		c.Version = version
	}

	if err := kafka.ConfigureAuthentication(config.Authentication, c); err != nil {
		return nil, err
	}

	client, err := sarama.NewConsumerGroup(config.Brokers, config.GroupID, c)
	if err != nil {
		return nil, err
	}
	return &kafkaTracesConsumer{
		consumerGroup:     client,
		topics:            []string{config.Topic},
		nextConsumer:      nextConsumer,
		unmarshaler:       unmarshaler,
		settings:          set,
		autocommitEnabled: config.AutoCommit.Enable,
		batchSize:         config.BatchSize,
		batchTimeout:      config.BatchTimeout,
	}, nil
}

func (c *kafkaTracesConsumer) Start(_ context.Context, host component.Host) error {
	ctx, cancel := context.WithCancel(context.Background())
	c.cancelConsumeLoop = cancel

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             c.settings.ID,
		Transport:              transport,
		ReceiverCreateSettings: c.settings,
	})
	if err != nil {
		return err
	}
	consumerGroup := &tracesConsumerGroupHandler{
		id:                c.settings.ID,
		logger:            c.settings.Logger,
		unmarshaler:       c.unmarshaler,
		nextConsumer:      c.nextConsumer,
		ready:             make(chan bool),
		obsrecv:           obsrecv,
		autocommitEnabled: c.autocommitEnabled,
		batchSize:         c.batchSize,
		batchTimeout:      c.batchTimeout,
	}
	go func() {
		if err := c.consumeLoop(ctx, consumerGroup); err != nil {
			c.settings.ReportStatus(component.NewFatalErrorEvent(err))
		}
	}()
	<-consumerGroup.ready
	return nil
}

func (c *kafkaTracesConsumer) consumeLoop(ctx context.Context, handler sarama.ConsumerGroupHandler) error {
	for {
		// `Consume` should be called inside an infinite loop, when a
		// server-side rebalance happens, the consumer session will need to be
		// recreated to get the new claims
		if err := c.consumerGroup.Consume(ctx, c.topics, handler); err != nil {
			c.settings.Logger.Error("Error from consumer", zap.Error(err))
		}
		// check if context was cancelled, signaling that the consumer should stop
		if ctx.Err() != nil {
			c.settings.Logger.Info("Consumer stopped", zap.Error(ctx.Err()))
			return ctx.Err()
		}
	}
}

func (c *kafkaTracesConsumer) Shutdown(context.Context) error {
	c.cancelConsumeLoop()
	return c.consumerGroup.Close()
}

func newMetricsReceiver(config Config, set receiver.CreateSettings, unmarshalers map[string]MetricsUnmarshaler, nextConsumer consumer.Metrics) (*kafkaMetricsConsumer, error) {
	unmarshaler := unmarshalers[config.Encoding]
	if unmarshaler == nil {
		return nil, errUnrecognizedEncoding
	}

	// set sarama library's logger to get detailed logs from the library
	sarama.Logger = zap.NewStdLog(set.Logger)

	c := sarama.NewConfig()
	c = setSaramaConsumerConfig(c, &config.SaramaConsumerConfig)
	c.ClientID = config.ClientID
	c.Metadata.Full = config.Metadata.Full
	c.Metadata.Retry.Max = config.Metadata.Retry.Max
	c.Metadata.Retry.Backoff = config.Metadata.Retry.Backoff
	c.Consumer.Offsets.AutoCommit.Enable = config.AutoCommit.Enable
	c.Consumer.Offsets.AutoCommit.Interval = config.AutoCommit.Interval

	if initialOffset, err := toSaramaInitialOffset(config.InitialOffset); err == nil {
		c.Consumer.Offsets.Initial = initialOffset
	} else {
		return nil, err
	}

	if config.ProtocolVersion != "" {
		version, err := sarama.ParseKafkaVersion(config.ProtocolVersion)
		if err != nil {
			return nil, err
		}
		c.Version = version
	}

	if err := kafka.ConfigureAuthentication(config.Authentication, c); err != nil {
		return nil, err
	}

	client, err := sarama.NewConsumerGroup(config.Brokers, config.GroupID, c)
	if err != nil {
		return nil, err
	}
	return &kafkaMetricsConsumer{
		consumerGroup:     client,
		topics:            []string{config.Topic},
		nextConsumer:      nextConsumer,
		unmarshaler:       unmarshaler,
		settings:          set,
		autocommitEnabled: config.AutoCommit.Enable,
		batchSize:         config.BatchSize,
		batchTimeout:      config.BatchTimeout,
	}, nil
}

func (c *kafkaMetricsConsumer) Start(_ context.Context, host component.Host) error {
	ctx, cancel := context.WithCancel(context.Background())
	c.cancelConsumeLoop = cancel
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             c.settings.ID,
		Transport:              transport,
		ReceiverCreateSettings: c.settings,
	})
	if err != nil {
		return err
	}
	metricsConsumerGroup := &metricsConsumerGroupHandler{
		id:                c.settings.ID,
		logger:            c.settings.Logger,
		unmarshaler:       c.unmarshaler,
		nextConsumer:      c.nextConsumer,
		ready:             make(chan bool),
		obsrecv:           obsrecv,
		autocommitEnabled: c.autocommitEnabled,
		batchSize:         c.batchSize,
		batchTimeout:      c.batchTimeout,
	}
	go func() {
		if err := c.consumeLoop(ctx, metricsConsumerGroup); err != nil {
			c.settings.ReportStatus(component.NewFatalErrorEvent(err))
		}
	}()
	<-metricsConsumerGroup.ready
	return nil
}

func (c *kafkaMetricsConsumer) consumeLoop(ctx context.Context, handler sarama.ConsumerGroupHandler) error {
	for {
		// `Consume` should be called inside an infinite loop, when a
		// server-side rebalance happens, the consumer session will need to be
		// recreated to get the new claims
		if err := c.consumerGroup.Consume(ctx, c.topics, handler); err != nil {
			c.settings.Logger.Error("Error from consumer", zap.Error(err))
		}
		// check if context was cancelled, signaling that the consumer should stop
		if ctx.Err() != nil {
			c.settings.Logger.Info("Consumer stopped", zap.Error(ctx.Err()))
			return ctx.Err()
		}
	}
}

func (c *kafkaMetricsConsumer) Shutdown(context.Context) error {
	c.cancelConsumeLoop()
	return c.consumerGroup.Close()
}

func newLogsReceiver(config Config, set receiver.CreateSettings, unmarshalers map[string]LogsUnmarshaler, nextConsumer consumer.Logs) (*kafkaLogsConsumer, error) {
	// set sarama library's logger to get detailed logs from the library
	sarama.Logger = zap.NewStdLog(set.Logger)

	c := sarama.NewConfig()
	c = setSaramaConsumerConfig(c, &config.SaramaConsumerConfig)
	c.ClientID = config.ClientID
	c.Metadata.Full = config.Metadata.Full
	c.Metadata.Retry.Max = config.Metadata.Retry.Max
	c.Metadata.Retry.Backoff = config.Metadata.Retry.Backoff
	c.Consumer.Offsets.AutoCommit.Enable = config.AutoCommit.Enable
	c.Consumer.Offsets.AutoCommit.Interval = config.AutoCommit.Interval

	if initialOffset, err := toSaramaInitialOffset(config.InitialOffset); err == nil {
		c.Consumer.Offsets.Initial = initialOffset
	} else {
		return nil, err
	}
	unmarshaler, err := getLogsUnmarshaler(config.Encoding, unmarshalers)
	if err != nil {
		return nil, err
	}

	if config.ProtocolVersion != "" {
		var version sarama.KafkaVersion
		version, err = sarama.ParseKafkaVersion(config.ProtocolVersion)
		if err != nil {
			return nil, err
		}
		c.Version = version
	}

	if err = kafka.ConfigureAuthentication(config.Authentication, c); err != nil {
		return nil, err
	}

	client, err := sarama.NewConsumerGroup(config.Brokers, config.GroupID, c)
	if err != nil {
		return nil, err
	}
	return &kafkaLogsConsumer{
		consumerGroup:     client,
		topics:            []string{config.Topic},
		nextConsumer:      nextConsumer,
		unmarshaler:       unmarshaler,
		settings:          set,
		autocommitEnabled: config.AutoCommit.Enable,
		batchSize:         config.BatchSize,
		batchTimeout:      config.BatchTimeout,
	}, nil
}

func getLogsUnmarshaler(encoding string, unmarshalers map[string]LogsUnmarshaler) (LogsUnmarshaler, error) {
	var enc string
	unmarshaler, ok := unmarshalers[encoding]
	if !ok {
		split := strings.SplitN(encoding, "_", 2)
		prefix := split[0]
		if len(split) > 1 {
			enc = split[1]
		}
		unmarshaler, ok = unmarshalers[prefix].(LogsUnmarshalerWithEnc)
		if !ok {
			return nil, errUnrecognizedEncoding
		}
	}

	if unmarshalerWithEnc, ok := unmarshaler.(LogsUnmarshalerWithEnc); ok {
		// This should be called even when enc is an empty string to initialize the encoding.
		unmarshaler, err := unmarshalerWithEnc.WithEnc(enc)
		if err != nil {
			return nil, err
		}
		return unmarshaler, nil
	}

	return unmarshaler, nil
}

func (c *kafkaLogsConsumer) Start(_ context.Context, host component.Host) error {
	ctx, cancel := context.WithCancel(context.Background())
	c.cancelConsumeLoop = cancel
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             c.settings.ID,
		Transport:              transport,
		ReceiverCreateSettings: c.settings,
	})
	if err != nil {
		return err
	}

	logsConsumerGroup := &logsConsumerGroupHandler{
		id:                c.settings.ID,
		logger:            c.settings.Logger,
		unmarshaler:       c.unmarshaler,
		nextConsumer:      c.nextConsumer,
		ready:             make(chan bool),
		obsrecv:           obsrecv,
		autocommitEnabled: c.autocommitEnabled,
		batchSize:         c.batchSize,
		batchTimeout:      c.batchTimeout,
	}
	go func() {
		if err := c.consumeLoop(ctx, logsConsumerGroup); err != nil {
			c.settings.ReportStatus(component.NewFatalErrorEvent(err))
		}
	}()
	<-logsConsumerGroup.ready
	return nil
}

func (c *kafkaLogsConsumer) consumeLoop(ctx context.Context, handler sarama.ConsumerGroupHandler) error {
	for {
		// `Consume` should be called inside an infinite loop, when a
		// server-side rebalance happens, the consumer session will need to be
		// recreated to get the new claims
		if err := c.consumerGroup.Consume(ctx, c.topics, handler); err != nil {
			c.settings.Logger.Error("Error from consumer", zap.Error(err))
		}
		// check if context was cancelled, signaling that the consumer should stop
		if ctx.Err() != nil {
			c.settings.Logger.Info("Consumer stopped", zap.Error(ctx.Err()))
			return ctx.Err()
		}
	}
}

func (c *kafkaLogsConsumer) Shutdown(context.Context) error {
	c.cancelConsumeLoop()
	return c.consumerGroup.Close()
}

type tracesConsumerGroupHandler struct {
	id           component.ID
	unmarshaler  TracesUnmarshaler
	nextConsumer consumer.Traces
	ready        chan bool
	readyCloser  sync.Once

	logger *zap.Logger

	obsrecv *receiverhelper.ObsReport

	autocommitEnabled bool

	// batching related fields
	batch          ptrace.Traces
	batchMutex     sync.Mutex
	batchTimer     *time.Timer
	batchItemCount int
	batchSize      int
	batchTimeout   time.Duration

	// field to track offsets
	offsetTracker map[string]map[int32]int64

	lastErr error
}

type metricsConsumerGroupHandler struct {
	id           component.ID
	unmarshaler  MetricsUnmarshaler
	nextConsumer consumer.Metrics
	ready        chan bool
	readyCloser  sync.Once

	logger *zap.Logger

	obsrecv *receiverhelper.ObsReport

	autocommitEnabled bool

	// batching related fields
	batch          pmetric.Metrics
	batchMutex     sync.Mutex
	batchTimer     *time.Timer
	batchItemCount int
	batchSize      int
	batchTimeout   time.Duration

	// field to track offsets
	offsetTracker map[string]map[int32]int64

	lastErr error
}

type logsConsumerGroupHandler struct {
	id           component.ID
	unmarshaler  LogsUnmarshaler
	nextConsumer consumer.Logs
	ready        chan bool
	readyCloser  sync.Once

	logger *zap.Logger

	obsrecv *receiverhelper.ObsReport

	autocommitEnabled bool

	// batching related fields
	batch          plog.Logs
	batchMutex     sync.Mutex
	batchTimer     *time.Timer
	batchItemCount int
	batchSize      int
	batchTimeout   time.Duration

	// field to track offsets
	offsetTracker map[string]map[int32]int64

	lastErr error
}

var _ sarama.ConsumerGroupHandler = (*tracesConsumerGroupHandler)(nil)
var _ sarama.ConsumerGroupHandler = (*metricsConsumerGroupHandler)(nil)
var _ sarama.ConsumerGroupHandler = (*logsConsumerGroupHandler)(nil)

func (c *tracesConsumerGroupHandler) Setup(session sarama.ConsumerGroupSession) error {
	c.readyCloser.Do(func() {
		close(c.ready)
	})
	statsTags := []tag.Mutator{tag.Upsert(tagInstanceName, c.id.Name())}
	_ = stats.RecordWithTags(session.Context(), statsTags, statPartitionStart.M(1))
	return nil
}

func (c *tracesConsumerGroupHandler) Cleanup(session sarama.ConsumerGroupSession) error {
	statsTags := []tag.Mutator{tag.Upsert(tagInstanceName, c.id.Name())}
	_ = stats.RecordWithTags(session.Context(), statsTags, statPartitionClose.M(1))
	return nil
}

func (c *tracesConsumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	c.logger.Info("Starting consumer group", zap.Int32("partition", claim.Partition()))
	c.batch = ptrace.NewTraces()
	c.batchTimer = time.NewTimer(c.batchTimeout)
	defer func() {
		if !c.batchTimer.Stop() {
			<-c.batchTimer.C
		}
	}()

	for {
		select {
		case message, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			return c.processTracesMessage(session, message)
		case <-c.batchTimer.C:
			return c.sendTracesBatch(session)
		// Should return when `session.Context()` is done.
		// If not, will raise `ErrRebalanceInProgress` or `read tcp <ip>:<port>: i/o timeout` when kafka rebalance. see:
		// https://github.com/Shopify/sarama/issues/1192
		case <-session.Context().Done():
			return nil
		}
	}
}

func (h *tracesConsumerGroupHandler) markOffsets(session sarama.ConsumerGroupSession) {
	for topic, partitions := range h.offsetTracker {
		for partition, offset := range partitions {
			session.MarkOffset(topic, partition, offset+1, "")
		}
	}
}

func (h *tracesConsumerGroupHandler) trackOffset(message *sarama.ConsumerMessage) {
	topic := message.Topic
	partition := message.Partition
	if _, exists := h.offsetTracker[topic]; !exists {
		h.offsetTracker[topic] = make(map[int32]int64)
	}
	if message.Offset > h.offsetTracker[topic][partition] {
		h.offsetTracker[topic][partition] = message.Offset
	}
}

func (h *tracesConsumerGroupHandler) processTracesMessage(session sarama.ConsumerGroupSession, message *sarama.ConsumerMessage) error {
	if h.lastErr != nil {
		// we don't want to consume the message if we failed to send the previous batch
		return h.lastErr
	}

	traces, err := h.unmarshaler.Unmarshal(message.Value)
	if err != nil {
		h.logger.Error("Failed to unmarshal traces message", zap.Error(err))
		return err
	}

	h.batchMutex.Lock()
	defer h.batchMutex.Unlock()

	traces.ResourceSpans().MoveAndAppendTo(h.batch.ResourceSpans())
	h.batchItemCount += traces.SpanCount()
	h.trackOffset(message)

	if h.batchItemCount >= h.batchSize {
		return h.sendTracesBatch(session)
	}
	return nil
}

func (h *tracesConsumerGroupHandler) sendTracesBatch(session sarama.ConsumerGroupSession) error {
	h.batchMutex.Lock()
	defer h.batchMutex.Unlock()

	if h.batchItemCount == 0 {
		return nil
	}

	ctx := h.obsrecv.StartTracesOp(session.Context())
	err := h.nextConsumer.ConsumeTraces(ctx, h.batch)
	h.lastErr = err
	h.obsrecv.EndTracesOp(ctx, h.unmarshaler.Encoding(), h.batchItemCount, err)

	if err != nil {
		h.logger.Error("Failed to consume traces", zap.Error(err))
		// we don't want to commit the offset if we failed to consume the traces
		return err
	}

	h.markOffsets(session)

	h.batch = ptrace.NewTraces()
	h.batchItemCount = 0
	h.batchTimer.Reset(h.batchTimeout)
	return nil
}

func (c *metricsConsumerGroupHandler) Setup(session sarama.ConsumerGroupSession) error {
	c.readyCloser.Do(func() {
		close(c.ready)
	})
	statsTags := []tag.Mutator{tag.Upsert(tagInstanceName, c.id.Name())}
	_ = stats.RecordWithTags(session.Context(), statsTags, statPartitionStart.M(1))
	return nil
}

func (c *metricsConsumerGroupHandler) Cleanup(session sarama.ConsumerGroupSession) error {
	statsTags := []tag.Mutator{tag.Upsert(tagInstanceName, c.id.Name())}
	_ = stats.RecordWithTags(session.Context(), statsTags, statPartitionClose.M(1))
	return nil
}

func (c *metricsConsumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	c.logger.Info("Starting consumer group", zap.Int32("partition", claim.Partition()))
	c.batch = pmetric.NewMetrics()
	c.batchTimer = time.NewTimer(c.batchTimeout)
	defer func() {
		if !c.batchTimer.Stop() {
			<-c.batchTimer.C
		}
	}()

	for {
		select {
		case message, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			return c.processMetricsMessage(session, message)
		case <-c.batchTimer.C:
			return c.sendMetricsBatch(session)
		// Should return when `session.Context()` is done.
		// If not, will raise `ErrRebalanceInProgress` or `read tcp <ip>:<port>: i/o timeout` when kafka rebalance. see:
		// https://github.com/Shopify/sarama/issues/1192
		case <-session.Context().Done():
			return nil
		}
	}
}

func (h *metricsConsumerGroupHandler) markOffsets(session sarama.ConsumerGroupSession) {
	for topic, partitions := range h.offsetTracker {
		for partition, offset := range partitions {
			session.MarkOffset(topic, partition, offset+1, "")
		}
	}
}

func (h *metricsConsumerGroupHandler) trackOffset(message *sarama.ConsumerMessage) {
	topic := message.Topic
	partition := message.Partition
	if _, exists := h.offsetTracker[topic]; !exists {
		h.offsetTracker[topic] = make(map[int32]int64)
	}
	if message.Offset > h.offsetTracker[topic][partition] {
		h.offsetTracker[topic][partition] = message.Offset
	}
}

func (h *metricsConsumerGroupHandler) processMetricsMessage(session sarama.ConsumerGroupSession, message *sarama.ConsumerMessage) error {
	if h.lastErr != nil {
		// we don't want to consume the message if we failed to send the previous batch
		return h.lastErr
	}

	metrics, err := h.unmarshaler.Unmarshal(message.Value)
	if err != nil {
		h.logger.Error("Failed to unmarshal traces message", zap.Error(err))
		return err
	}

	h.batchMutex.Lock()
	defer h.batchMutex.Unlock()

	metrics.ResourceMetrics().MoveAndAppendTo(h.batch.ResourceMetrics())
	h.batchItemCount += metrics.MetricCount()
	h.trackOffset(message)

	if h.batchItemCount >= h.batchSize {
		return h.sendMetricsBatch(session)
	}
	return nil
}

func (h *metricsConsumerGroupHandler) sendMetricsBatch(session sarama.ConsumerGroupSession) error {
	h.batchMutex.Lock()
	defer h.batchMutex.Unlock()

	if h.batchItemCount == 0 {
		return nil
	}

	ctx := h.obsrecv.StartMetricsOp(session.Context())
	err := h.nextConsumer.ConsumeMetrics(ctx, h.batch)
	h.lastErr = err
	h.obsrecv.EndMetricsOp(ctx, h.unmarshaler.Encoding(), h.batchItemCount, err)

	if err != nil {
		h.logger.Error("Failed to consume metrics", zap.Error(err))
		// we don't want to commit the offset if we failed to consume the metrics
		return err
	}

	h.markOffsets(session)

	h.batch = pmetric.NewMetrics()
	h.batchItemCount = 0
	h.batchTimer.Reset(h.batchTimeout)
	return nil
}

func (c *logsConsumerGroupHandler) Setup(session sarama.ConsumerGroupSession) error {
	c.readyCloser.Do(func() {
		close(c.ready)
	})
	_ = stats.RecordWithTags(
		session.Context(),
		[]tag.Mutator{tag.Upsert(tagInstanceName, c.id.String())},
		statPartitionStart.M(1))
	return nil
}

func (c *logsConsumerGroupHandler) Cleanup(session sarama.ConsumerGroupSession) error {
	_ = stats.RecordWithTags(
		session.Context(),
		[]tag.Mutator{tag.Upsert(tagInstanceName, c.id.String())},
		statPartitionClose.M(1))
	return nil
}

func (c *logsConsumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	c.logger.Info("Starting consumer group", zap.Int32("partition", claim.Partition()))
	c.batch = plog.NewLogs()
	c.batchTimer = time.NewTimer(c.batchTimeout)
	defer func() {
		if !c.batchTimer.Stop() {
			<-c.batchTimer.C
		}
	}()

	for {
		select {
		case message, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			return c.processLogsMessage(session, message)
		case <-c.batchTimer.C:
			return c.sendLogsBatch(session)
		// Should return when `session.Context()` is done.
		// If not, will raise `ErrRebalanceInProgress` or `read tcp <ip>:<port>: i/o timeout` when kafka rebalance. see:
		// https://github.com/Shopify/sarama/issues/1192
		case <-session.Context().Done():
			return nil
		}
	}
}

func (h *logsConsumerGroupHandler) markOffsets(session sarama.ConsumerGroupSession) {
	for topic, partitions := range h.offsetTracker {
		for partition, offset := range partitions {
			session.MarkOffset(topic, partition, offset+1, "")
		}
	}
}

func (h *logsConsumerGroupHandler) trackOffset(message *sarama.ConsumerMessage) {
	topic := message.Topic
	partition := message.Partition
	if _, exists := h.offsetTracker[topic]; !exists {
		h.offsetTracker[topic] = make(map[int32]int64)
	}
	if message.Offset > h.offsetTracker[topic][partition] {
		h.offsetTracker[topic][partition] = message.Offset
	}
}

func (h *logsConsumerGroupHandler) processLogsMessage(session sarama.ConsumerGroupSession, message *sarama.ConsumerMessage) error {
	if h.lastErr != nil {
		// we don't want to consume the message if we failed to send the previous batch
		return h.lastErr
	}

	logs, err := h.unmarshaler.Unmarshal(message.Value)
	if err != nil {
		h.logger.Error("Failed to unmarshal traces message", zap.Error(err))
		return err
	}

	h.batchMutex.Lock()
	defer h.batchMutex.Unlock()

	logs.ResourceLogs().MoveAndAppendTo(h.batch.ResourceLogs())
	h.batchItemCount += logs.LogRecordCount()
	h.trackOffset(message)

	if h.batchItemCount >= h.batchSize {
		return h.sendLogsBatch(session)
	}
	return nil
}

func (h *logsConsumerGroupHandler) sendLogsBatch(session sarama.ConsumerGroupSession) error {
	h.batchMutex.Lock()
	defer h.batchMutex.Unlock()

	if h.batchItemCount == 0 {
		return nil
	}

	ctx := h.obsrecv.StartLogsOp(session.Context())
	err := h.nextConsumer.ConsumeLogs(ctx, h.batch)
	h.lastErr = err
	h.obsrecv.EndLogsOp(ctx, h.unmarshaler.Encoding(), h.batchItemCount, err)

	if err != nil {
		h.logger.Error("Failed to consume logs", zap.Error(err))
		// we don't want to commit the offset if we failed to consume the logs
		return err
	}

	h.markOffsets(session)

	h.batch = plog.NewLogs()
	h.batchItemCount = 0
	h.batchTimer.Reset(h.batchTimeout)
	return nil
}

func toSaramaInitialOffset(initialOffset string) (int64, error) {
	switch initialOffset {
	case offsetEarliest:
		return sarama.OffsetOldest, nil
	case offsetLatest:
		fallthrough
	case "":
		return sarama.OffsetNewest, nil
	default:
		return 0, errInvalidInitialOffset
	}
}

func setSaramaConsumerConfig(sc *sarama.Config, c *SaramaConsumerConfig) *sarama.Config {
	if c.ConsumerFetchMinBytes != 0 {
		sc.Consumer.Fetch.Min = c.ConsumerFetchMinBytes
	}
	if c.ConsumerFetchDefaultBytes != 0 {
		sc.Consumer.Fetch.Default = c.ConsumerFetchDefaultBytes
	}
	if c.ConsumerFetchMaxBytes != 0 {
		sc.Consumer.Fetch.Max = c.ConsumerFetchMaxBytes
		if c.ConsumerFetchMaxBytes > sarama.MaxResponseSize {
			// If the user has set a Consumer.Fetch.Max that is larger than the MaxResponseSize, then
			// set the MaxResponseSize to the Consumer.Fetch.Max.
			// If we dont do this, then sarama will reject messages > 100MB with a packet decoding error.
			sarama.MaxResponseSize = c.ConsumerFetchMaxBytes
		}
	}
	if c.MaxProcessingTime != 0 {
		sc.Consumer.MaxProcessingTime = c.MaxProcessingTime
	}
	if c.GroupSessionTimeout != 0 {
		sc.Consumer.Group.Session.Timeout = c.GroupSessionTimeout
	}
	if c.MessagesChannelSize != 0 {
		sc.ChannelBufferSize = c.MessagesChannelSize
	}
	return sc
}

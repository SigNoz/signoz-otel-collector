// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signozkafkareceiver // import "github.com/SigNoz/signoz-otel-collector/receiver/signozkafkareceiver"

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/SigNoz/signoz-otel-collector/internal/kafka"
	"github.com/SigNoz/signoz-otel-collector/receiver/signozkafkareceiver/internal/metadata"
)

const (
	transport = "kafka"
)

var errUnrecognizedEncoding = fmt.Errorf("unrecognized encoding")
var errInvalidInitialOffset = fmt.Errorf("invalid initial offset")

// errHighMemoryUsage is a sentinel error for high memory usage
var errHighMemoryUsage = errors.New("data refused due to high memory usage")

// kafkaTracesConsumer uses sarama to consume and handle messages from kafka.
type kafkaTracesConsumer struct {
	consumerGroup     sarama.ConsumerGroup
	nextConsumer      consumer.Traces
	topics            []string
	cancelConsumeLoop context.CancelFunc
	unmarshaler       TracesUnmarshaler

	settings receiver.Settings

	autocommitEnabled bool
	messageMarking    MessageMarking
}

// kafkaMetricsConsumer uses sarama to consume and handle messages from kafka.
type kafkaMetricsConsumer struct {
	consumerGroup     sarama.ConsumerGroup
	nextConsumer      consumer.Metrics
	topics            []string
	cancelConsumeLoop context.CancelFunc
	unmarshaler       MetricsUnmarshaler

	settings receiver.Settings

	autocommitEnabled bool
	messageMarking    MessageMarking
}

// kafkaLogsConsumer uses sarama to consume and handle messages from kafka.
type kafkaLogsConsumer struct {
	consumerGroup     sarama.ConsumerGroup
	nextConsumer      consumer.Logs
	topics            []string
	cancelConsumeLoop context.CancelFunc
	unmarshaler       LogsUnmarshaler

	settings receiver.Settings

	autocommitEnabled bool
	messageMarking    MessageMarking
}

var _ receiver.Traces = (*kafkaTracesConsumer)(nil)
var _ receiver.Metrics = (*kafkaMetricsConsumer)(nil)
var _ receiver.Logs = (*kafkaLogsConsumer)(nil)

func newTracesReceiver(config Config, set receiver.Settings, unmarshalers map[string]TracesUnmarshaler, nextConsumer consumer.Traces) (*kafkaTracesConsumer, error) {
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
		messageMarking:    config.MessageMarking,
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
	metrics, err := NewKafkaReceiverMetrics(c.settings.MeterProvider.Meter(metadata.ScopeName))
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
		messageMarking:    c.messageMarking,
		baseConsumerGroupHandler: baseConsumerGroupHandler{
			group:           c.consumerGroup,
			logger:          c.settings.Logger,
			retryInterval:   1 * time.Second,
			pausePartition:  make(chan struct{}),
			resumePartition: make(chan struct{}),
			metrics:         metrics,
		},
	}
	go func() {
		if err := c.consumeLoop(ctx, consumerGroup); !errors.Is(err, context.Canceled) {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(err))
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

func newMetricsReceiver(config Config, set receiver.Settings, unmarshalers map[string]MetricsUnmarshaler, nextConsumer consumer.Metrics) (*kafkaMetricsConsumer, error) {
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
		messageMarking:    config.MessageMarking,
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

	metrics, err := NewKafkaReceiverMetrics(c.settings.MeterProvider.Meter(metadata.ScopeName))
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
		messageMarking:    c.messageMarking,
		baseConsumerGroupHandler: baseConsumerGroupHandler{
			group:           c.consumerGroup,
			logger:          c.settings.Logger,
			retryInterval:   1 * time.Second,
			pausePartition:  make(chan struct{}),
			resumePartition: make(chan struct{}),
			metrics:         metrics,
		},
	}
	go func() {
		if err := c.consumeLoop(ctx, metricsConsumerGroup); !errors.Is(err, context.Canceled) {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(err))
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

func newLogsReceiver(config Config, set receiver.Settings, unmarshalers map[string]LogsUnmarshaler, nextConsumer consumer.Logs) (*kafkaLogsConsumer, error) {
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
		messageMarking:    config.MessageMarking,
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

	metrics, err := NewKafkaReceiverMetrics(c.settings.MeterProvider.Meter(metadata.ScopeName))
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
		messageMarking:    c.messageMarking,
		baseConsumerGroupHandler: baseConsumerGroupHandler{
			group:           c.consumerGroup,
			logger:          c.settings.Logger,
			retryInterval:   1 * time.Second,
			pausePartition:  make(chan struct{}),
			resumePartition: make(chan struct{}),
			metrics:         metrics,
		},
	}
	go func() {
		if err := c.consumeLoop(ctx, logsConsumerGroup); !errors.Is(err, context.Canceled) {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(err))
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

type baseConsumerGroupHandler struct {
	group           sarama.ConsumerGroup
	logger          *zap.Logger
	retryInterval   time.Duration
	pausePartition  chan struct{}
	resumePartition chan struct{}

	metrics *KafkaReceiverMetrics
}

// wrap is now a method of BaseConsumerGroupHandler
func (b *baseConsumerGroupHandler) WithMemoryLimiter(ctx context.Context, claim sarama.ConsumerGroupClaim, consume func() error) error {
	// Execute f() immediately
	err := consume()
	if err == nil || !(err.Error() == errHighMemoryUsage.Error()) {
		return err
	}

	// If errHighMemoryUsage is encountered, enter the retry loop
	ticker := time.NewTicker(b.retryInterval)
	defer ticker.Stop()

	b.logger.Info("applying initial backpressure on Kafka due to high memory usage", zap.Int32("partition", claim.Partition()), zap.String("topic", claim.Topic()))
	b.group.PauseAll()

	for {
		select {
		case <-ticker.C:
			err := consume()
			if err == nil {
				b.logger.Info("resuming normal operation", zap.Int32("partition", claim.Partition()), zap.String("topic", claim.Topic()))
				b.group.ResumeAll()
				return nil
			}
			if !(err.Error() == errHighMemoryUsage.Error()) {
				b.group.ResumeAll()
				return err
			}
			b.logger.Info("continuing to apply backpressure on Kafka due to high memory usage", zap.Int32("partition", claim.Partition()), zap.String("topic", claim.Topic()))
		case <-ctx.Done():
			if ctx.Err() != nil {
				b.logger.Error("session ended before message could be processed", zap.Error(ctx.Err()))
			}
			return ctx.Err()
		}
	}
}

type tracesConsumerGroupHandler struct {
	baseConsumerGroupHandler
	id           component.ID
	unmarshaler  TracesUnmarshaler
	nextConsumer consumer.Traces
	ready        chan bool
	readyCloser  sync.Once

	logger *zap.Logger

	obsrecv *receiverhelper.ObsReport

	autocommitEnabled bool
	messageMarking    MessageMarking
}

type metricsConsumerGroupHandler struct {
	baseConsumerGroupHandler
	id           component.ID
	unmarshaler  MetricsUnmarshaler
	nextConsumer consumer.Metrics
	ready        chan bool
	readyCloser  sync.Once

	logger *zap.Logger

	obsrecv *receiverhelper.ObsReport

	autocommitEnabled bool
	messageMarking    MessageMarking
}

type logsConsumerGroupHandler struct {
	baseConsumerGroupHandler
	id           component.ID
	unmarshaler  LogsUnmarshaler
	nextConsumer consumer.Logs
	ready        chan bool
	readyCloser  sync.Once

	logger *zap.Logger

	obsrecv *receiverhelper.ObsReport

	autocommitEnabled bool
	messageMarking    MessageMarking
}

var _ sarama.ConsumerGroupHandler = (*tracesConsumerGroupHandler)(nil)
var _ sarama.ConsumerGroupHandler = (*metricsConsumerGroupHandler)(nil)
var _ sarama.ConsumerGroupHandler = (*logsConsumerGroupHandler)(nil)

func (c *tracesConsumerGroupHandler) Setup(session sarama.ConsumerGroupSession) error {
	c.readyCloser.Do(func() {
		close(c.ready)
	})
	c.metrics.partitionStart.Add(
		session.Context(),
		1,
		metric.WithAttributes(attribute.String("name", c.id.String())),
	)
	return nil
}

func (c *tracesConsumerGroupHandler) Cleanup(session sarama.ConsumerGroupSession) error {
	c.metrics.partitionClose.Add(
		session.Context(),
		1,
		metric.WithAttributes(attribute.String("name", c.id.String())),
	)
	return nil
}

func (c *tracesConsumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	c.logger.Info("Starting consumer group", zap.Int32("partition", claim.Partition()))
	if !c.autocommitEnabled {
		defer session.Commit()
	}
	for {
		select {
		case message, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			start := time.Now()
			c.logger.Debug("Kafka message claimed",
				zap.String("value", string(message.Value)),
				zap.Time("timestamp", message.Timestamp),
				zap.String("topic", message.Topic),
				zap.Int32("partition", message.Partition),
				zap.Int64("offset", message.Offset),
			)
			if !c.messageMarking.After {
				session.MarkMessage(message, "")
			}

			ctx := c.obsrecv.StartTracesOp(session.Context())
			c.metrics.messagesCount.Add(
				session.Context(),
				1,
				metric.WithAttributes(attribute.String("name", c.id.String())),
			)
			c.metrics.messageOffset.Record(
				session.Context(),
				int64(message.Offset),
				metric.WithAttributes(attribute.String("name", c.id.String())),
			)
			c.metrics.messageOffsetLag.Record(
				session.Context(),
				int64(claim.HighWaterMarkOffset()-message.Offset-1),
				metric.WithAttributes(attribute.String("name", c.id.String())),
			)

			traces, err := c.unmarshaler.Unmarshal(message.Value)
			if err != nil {
				c.logger.Error("failed to unmarshal message", zap.Error(err))
				if c.messageMarking.After && c.messageMarking.OnError {
					session.MarkMessage(message, "")
				}
				return err
			}

			spanCount := traces.SpanCount()
			err = c.WithMemoryLimiter(session.Context(), claim, func() error { return c.nextConsumer.ConsumeTraces(session.Context(), traces) })
			c.obsrecv.EndTracesOp(ctx, c.unmarshaler.Encoding(), spanCount, err)
			if err != nil {
				c.logger.Error("kafka receiver: failed to export traces", zap.Error(err), zap.Int32("partition", claim.Partition()), zap.String("topic", claim.Topic()))
				if c.messageMarking.After && c.messageMarking.OnError {
					session.MarkMessage(message, "")
				}

				return err
			}

			if c.messageMarking.After {
				session.MarkMessage(message, "")
			}
			if !c.autocommitEnabled {
				session.Commit()
			}
			c.metrics.processingTime.Record(session.Context(), float64(time.Since(start).Milliseconds()))
			if err != nil {
				c.logger.Error("failed to record processing time", zap.Error(err))
			}

		// Should return when `session.Context()` is done.
		// If not, will raise `ErrRebalanceInProgress` or `read tcp <ip>:<port>: i/o timeout` when kafka rebalance. see:
		// https://github.com/Shopify/sarama/issues/1192
		case <-session.Context().Done():
			return nil
		}
	}
}

func (c *metricsConsumerGroupHandler) Setup(session sarama.ConsumerGroupSession) error {
	c.readyCloser.Do(func() {
		close(c.ready)
	})
	c.metrics.partitionStart.Add(
		session.Context(),
		1,
		metric.WithAttributes(attribute.String("name", c.id.String())),
	)
	return nil
}

func (c *metricsConsumerGroupHandler) Cleanup(session sarama.ConsumerGroupSession) error {
	c.metrics.partitionClose.Add(
		session.Context(),
		1,
		metric.WithAttributes(attribute.String("name", c.id.String())),
	)
	return nil
}

func (c *metricsConsumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	c.logger.Info("Starting consumer group", zap.Int32("partition", claim.Partition()))
	if !c.autocommitEnabled {
		defer session.Commit()
	}
	for {
		select {
		case message, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			start := time.Now()
			c.logger.Debug("Kafka message claimed",
				zap.String("value", string(message.Value)),
				zap.Time("timestamp", message.Timestamp),
				zap.String("topic", message.Topic))
			if !c.messageMarking.After {
				session.MarkMessage(message, "")
			}

			ctx := c.obsrecv.StartMetricsOp(session.Context())
			c.metrics.messagesCount.Add(
				session.Context(),
				1,
				metric.WithAttributes(attribute.String("name", c.id.String())),
			)
			c.metrics.messageOffset.Record(
				session.Context(),
				int64(message.Offset),
				metric.WithAttributes(attribute.String("name", c.id.String())),
			)
			c.metrics.messageOffsetLag.Record(
				session.Context(),
				int64(claim.HighWaterMarkOffset()-message.Offset-1),
				metric.WithAttributes(attribute.String("name", c.id.String())),
			)

			metrics, err := c.unmarshaler.Unmarshal(message.Value)
			if err != nil {
				c.logger.Error("failed to unmarshal message", zap.Error(err))
				if c.messageMarking.After && c.messageMarking.OnError {
					session.MarkMessage(message, "")
				}
				return err
			}

			dataPointCount := metrics.DataPointCount()
			err = c.WithMemoryLimiter(session.Context(), claim, func() error { return c.nextConsumer.ConsumeMetrics(session.Context(), metrics) })
			c.obsrecv.EndMetricsOp(ctx, c.unmarshaler.Encoding(), dataPointCount, err)
			if err != nil {
				c.logger.Error("kafka receiver: failed to export metrics", zap.Error(err), zap.Int32("partition", claim.Partition()), zap.String("topic", claim.Topic()))
				if c.messageMarking.After && c.messageMarking.OnError {
					session.MarkMessage(message, "")
				}
				return err
			}
			if c.messageMarking.After {
				session.MarkMessage(message, "")
			}
			if !c.autocommitEnabled {
				session.Commit()
			}
			c.metrics.processingTime.Record(
				session.Context(),
				float64(time.Since(start).Milliseconds()),
				metric.WithAttributes(attribute.String("name", c.id.String())),
			)
			if err != nil {
				c.logger.Error("failed to record processing time", zap.Error(err))
			}

		// Should return when `session.Context()` is done.
		// If not, will raise `ErrRebalanceInProgress` or `read tcp <ip>:<port>: i/o timeout` when kafka rebalance. see:
		// https://github.com/Shopify/sarama/issues/1192
		case <-session.Context().Done():
			return nil
		}
	}
}

func (c *logsConsumerGroupHandler) Setup(session sarama.ConsumerGroupSession) error {
	c.readyCloser.Do(func() {
		close(c.ready)
	})
	c.metrics.partitionStart.Add(
		session.Context(),
		1,
		metric.WithAttributes(attribute.String("name", c.id.String())),
	)
	return nil
}

func (c *logsConsumerGroupHandler) Cleanup(session sarama.ConsumerGroupSession) error {
	c.metrics.partitionClose.Add(
		session.Context(),
		1,
		metric.WithAttributes(attribute.String("name", c.id.String())),
	)
	return nil
}

func (c *logsConsumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	c.logger.Info("Starting consumer group", zap.Int32("partition", claim.Partition()))
	if !c.autocommitEnabled {
		defer session.Commit()
	}

	for {
		select {
		case message, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			start := time.Now()
			c.logger.Debug("Kafka message claimed",
				zap.String("value", string(message.Value)),
				zap.Time("timestamp", message.Timestamp),
				zap.String("topic", message.Topic),
				zap.Int32("partition", message.Partition),
				zap.Int64("offset", message.Offset))
			if !c.messageMarking.After {
				session.MarkMessage(message, "")
			}

			ctx := c.obsrecv.StartLogsOp(session.Context())
			c.metrics.messagesCount.Add(
				session.Context(),
				1,
				metric.WithAttributes(attribute.String("name", c.id.String())),
			)
			c.metrics.messageOffset.Record(
				session.Context(),
				int64(message.Offset),
				metric.WithAttributes(attribute.String("name", c.id.String())),
			)
			c.metrics.messageOffsetLag.Record(
				session.Context(),
				int64(claim.HighWaterMarkOffset()-message.Offset-1),
				metric.WithAttributes(attribute.String("name", c.id.String())),
			)

			logs, err := c.unmarshaler.Unmarshal(message.Value)
			if err != nil {
				c.logger.Error("failed to unmarshal message", zap.Error(err))
				if c.messageMarking.After && c.messageMarking.OnError {
					session.MarkMessage(message, "")
				}
				return err
			}

			logCount := logs.LogRecordCount()
			err = c.WithMemoryLimiter(session.Context(), claim, func() error { return c.nextConsumer.ConsumeLogs(session.Context(), logs) })
			c.obsrecv.EndLogsOp(ctx, c.unmarshaler.Encoding(), logCount, err)
			if err != nil {
				c.logger.Error("kafka receiver: failed to export logs", zap.Error(err), zap.Int32("partition", claim.Partition()), zap.String("topic", claim.Topic()))
				if c.messageMarking.After && c.messageMarking.OnError {
					session.MarkMessage(message, "")
				}
				return err
			}
			if c.messageMarking.After {
				session.MarkMessage(message, "")
			}
			if !c.autocommitEnabled {
				session.Commit()
			}
			c.metrics.processingTime.Record(
				session.Context(),
				float64(time.Since(start).Milliseconds()),
				metric.WithAttributes(attribute.String("name", c.id.String())),
			)
			if err != nil {
				c.logger.Error("failed to record processing time", zap.Error(err))
			}

		// Should return when `session.Context()` is done.
		// If not, will raise `ErrRebalanceInProgress` or `read tcp <ip>:<port>: i/o timeout` when kafka rebalance. see:
		// https://github.com/Shopify/sarama/issues/1192
		case <-session.Context().Done():
			return nil
		}
	}
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

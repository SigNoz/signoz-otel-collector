// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signozkafkareceiver // import "github.com/SigNoz/signoz-otel-collector/receiver/signozkafkareceiver"

import (
	"go.opentelemetry.io/otel/metric"
)

type KafkaReceiverMetrics struct {
	messagesCount    metric.Int64Counter
	messageOffset    metric.Int64Gauge
	messageOffsetLag metric.Int64Gauge

	partitionStart metric.Int64Counter
	partitionClose metric.Int64Counter

	processingTime metric.Float64Histogram
}

func NewKafkaReceiverMetrics(meter metric.Meter) (*KafkaReceiverMetrics, error) {
	messagesCount, err := meter.Int64Counter(
		"kafka_receiver_messages",
		metric.WithDescription("Number of received messages"),
	)
	if err != nil {
		return nil, err
	}
	messageOffset, err := meter.Int64Gauge(
		"kafka_receiver_current_offset",
		metric.WithDescription("Current message offset"),
	)
	if err != nil {
		return nil, err
	}
	messageOffsetLag, err := meter.Int64Gauge(
		"kafka_receiver_offset_lag",
		metric.WithDescription("Current offset lag"),
	)
	if err != nil {
		return nil, err
	}
	partitionStart, err := meter.Int64Counter(
		"kafka_receiver_partition_start",
		metric.WithDescription("Number of started partitions"),
	)
	if err != nil {
		return nil, err
	}
	partitionClose, err := meter.Int64Counter(
		"kafka_receiver_partition_close",
		metric.WithDescription("Number of finished partitions"),
	)
	if err != nil {
		return nil, err
	}

	processingTime, err := meter.Float64Histogram(
		"kafka_receiver_processing_time_milliseconds",
		metric.WithDescription("Time taken to process a kafka message in ms"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(250, 500, 750, 1000, 2000, 2500, 3000, 4000, 5000, 6000, 8000, 10000, 15000, 25000, 30000),
	)
	if err != nil {
		return nil, err
	}

	return &KafkaReceiverMetrics{
		messagesCount:    messagesCount,
		messageOffset:    messageOffset,
		messageOffsetLag: messageOffsetLag,
		partitionStart:   partitionStart,
		partitionClose:   partitionClose,
		processingTime:   processingTime,
	}, nil
}

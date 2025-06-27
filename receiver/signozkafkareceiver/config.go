// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signozkafkareceiver // import "github.com/SigNoz/signoz-otel-collector/receiver/signozkafkareceiver"

import (
	"time"

	"go.opentelemetry.io/collector/component"

	"github.com/SigNoz/signoz-otel-collector/internal/kafka"
)

type AutoCommit struct {
	// Whether or not to auto-commit updated offsets back to the broker.
	// (default enabled).
	Enable bool `mapstructure:"enable"`
	// How frequently to commit updated offsets. Ineffective unless
	// auto-commit is enabled (default 1s)
	Interval time.Duration `mapstructure:"interval"`
}

type MessageMarking struct {
	// If true, the messages are marked after the pipeline execution
	After bool `mapstructure:"after"`

	// If false, only the successfully processed messages are marked, it has no impact if
	// After is set to false.
	// Note: this can block the entire partition in case a message processing returns
	// a permanent error.
	OnError bool `mapstructure:"on_error"`
}

type SaramaConsumerConfig struct {
	// Controls sarama client's Consumer.Fetch config if set.
	ConsumerFetchMinBytes     int32 `mapstructure:"fetch_min_bytes"`
	ConsumerFetchDefaultBytes int32 `mapstructure:"fetch_default_bytes"`
	ConsumerFetchMaxBytes     int32 `mapstructure:"fetch_max_bytes"`

	MaxProcessingTime   time.Duration `mapstructure:"max_processing_time"`
	GroupSessionTimeout time.Duration `mapstructure:"consumer_group_session_timeout"`
	MessagesChannelSize int           `mapstructure:"messages_channel_size"`
}

// Config defines configuration for Kafka receiver.
type Config struct {
	// The list of kafka brokers (default localhost:9092)
	Brokers []string `mapstructure:"brokers"`
	// Kafka protocol version
	ProtocolVersion string `mapstructure:"protocol_version"`
	// The name of the kafka topic to consume from (default "otlp_spans")
	Topic string `mapstructure:"topic"`
	// Encoding of the messages (default "otlp_proto")
	Encoding string `mapstructure:"encoding"`
	// The consumer group that receiver will be consuming messages from (default "otel-collector")
	GroupID string `mapstructure:"group_id"`
	// The consumer client ID that receiver will use (default "otel-collector")
	ClientID string `mapstructure:"client_id"`
	// The initial offset to use if no offset was previously committed.
	// Must be `latest` or `earliest` (default "latest").
	InitialOffset string `mapstructure:"initial_offset"`

	// Metadata is the namespace for metadata management properties used by the
	// Client, and shared by the Producer/Consumer.
	Metadata Metadata `mapstructure:"metadata"`

	Authentication kafka.Authentication `mapstructure:"auth"`

	// Controls the auto-commit functionality
	AutoCommit AutoCommit `mapstructure:"autocommit"`

	// Controls the way the messages are marked as consumed
	MessageMarking MessageMarking `mapstructure:"message_marking"`

	SaramaConsumerConfig SaramaConsumerConfig `mapstructure:"sarama_consumer_config"`
}

// Metadata defines configuration for retrieving metadata from the broker.
//
// Note: directly imported due to upstream shifted into internal
type Metadata struct {
	// Whether to maintain a full set of metadata for all topics, or just
	// the minimal set that has been necessary so far. The full set is simpler
	// and usually more convenient, but can take up a substantial amount of
	// memory if you have many topics and partitions. Defaults to true.
	Full bool `mapstructure:"full"`

	// Retry configuration for metadata.
	// This configuration is useful to avoid race conditions when broker
	// is starting at the same time as collector.
	Retry MetadataRetry `mapstructure:"retry"`
}

// MetadataRetry defines retry configuration for Metadata.
//
// Note: directly imported due to upstream shifted into internal
type MetadataRetry struct {
	// The total number of times to retry a metadata request when the
	// cluster is in the middle of a leader election or at startup (default 3).
	Max int `mapstructure:"max"`
	// How long to wait for leader election to occur before retrying
	// (default 250ms). Similar to the JVM's `retry.backoff.ms`.
	Backoff time.Duration `mapstructure:"backoff"`
}

const (
	offsetLatest   string = "latest"
	offsetEarliest string = "earliest"
)

var _ component.Config = (*Config)(nil)

// Validate checks the receiver configuration is valid
func (cfg *Config) Validate() error {
	return nil
}

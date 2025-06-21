// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signozkafkaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"

import (
	"fmt"
	"time"

	"github.com/Shopify/sarama"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config defines configuration for Kafka exporter.
type Config struct {
	exporterhelper.TimeoutConfig `mapstructure:",squash"`        // squash ensures fields are correctly decoded in embedded struct.
	QueueBatchConfig             exporterhelper.QueueBatchConfig `mapstructure:"sending_queue"`
	BackOffConfig                configretry.BackOffConfig       `mapstructure:"retry_on_failure"`

	// The list of kafka brokers (default localhost:9092)
	Brokers []string `mapstructure:"brokers"`
	// Kafka protocol version
	ProtocolVersion string `mapstructure:"protocol_version"`
	// The name of the kafka topic to export to (default otlp_spans for traces, otlp_metrics for metrics)
	Topic string `mapstructure:"topic"`

	// Encoding of messages (default "otlp_proto")
	Encoding string `mapstructure:"encoding"`

	// Metadata is the namespace for metadata management properties used by the
	// Client, and shared by the Producer/Consumer.
	Metadata MetadataConfig `mapstructure:"metadata"`

	// Producer is the namespaces for producer properties used only by the Producer
	Producer Producer `mapstructure:"producer"`

	// Authentication defines used authentication mechanism.
	Authentication Authentication `mapstructure:"auth"`
}

type MetadataConfig struct {
	// Whether to maintain a full set of metadata for all topics, or just
	// the minimal set that has been necessary so far. The full set is simpler
	// and usually more convenient, but can take up a substantial amount of
	// memory if you have many topics and partitions. Defaults to true.
	Full bool `mapstructure:"full"`

	// RefreshInterval controls the frequency at which cluster metadata is
	// refreshed. Defaults to 10 minutes.
	RefreshInterval time.Duration `mapstructure:"refresh_interval"`

	// Retry configuration for metadata.
	// This configuration is useful to avoid race conditions when broker
	// is starting at the same time as collector.
	Retry MetadataRetryConfig `mapstructure:"retry"`
}

// Producer defines configuration for producer
type Producer struct {
	// Maximum message bytes the producer will accept to produce.
	MaxMessageBytes int `mapstructure:"max_message_bytes"`

	// RequiredAcks Number of acknowledgements required to assume that a message has been sent.
	// https://pkg.go.dev/github.com/Shopify/sarama@v1.30.0#RequiredAcks
	// The options are:
	//   0 -> NoResponse.  doesn't send any response
	//   1 -> WaitForLocal. waits for only the local commit to succeed before responding ( default )
	//   -1 -> WaitForAll. waits for all in-sync replicas to commit before responding.
	RequiredAcks sarama.RequiredAcks `mapstructure:"required_acks"`

	// Compression Codec used to produce messages
	// https://pkg.go.dev/github.com/Shopify/sarama@v1.30.0#CompressionCodec
	// The options are: 'none', 'gzip', 'snappy', 'lz4', and 'zstd'
	Compression string `mapstructure:"compression"`

	// The maximum number of messages the producer will send in a single
	// broker request. Defaults to 0 for unlimited. Similar to
	// `queue.buffering.max.messages` in the JVM producer.
	FlushMaxMessages int `mapstructure:"flush_max_messages"`
}

// MetadataRetryConfig defines retry configuration for Metadata.
type MetadataRetryConfig struct {
	// The total number of times to retry a metadata request when the
	// cluster is in the middle of a leader election or at startup (default 3).
	Max int `mapstructure:"max"`
	// How long to wait for leader election to occur before retrying
	// (default 250ms). Similar to the JVM's `retry.backoff.ms`.
	Backoff time.Duration `mapstructure:"backoff"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the exporter configuration is valid
func (cfg *Config) Validate() error {
	if cfg.Producer.RequiredAcks < -1 || cfg.Producer.RequiredAcks > 1 {
		return fmt.Errorf("producer.required_acks has to be between -1 and 1. configured value %v", cfg.Producer.RequiredAcks)
	}

	_, err := saramaProducerCompressionCodec(cfg.Producer.Compression)
	if err != nil {
		return err
	}

	return nil
}

func saramaProducerCompressionCodec(compression string) (sarama.CompressionCodec, error) {
	switch compression {
	case "none":
		return sarama.CompressionNone, nil
	case "gzip":
		return sarama.CompressionGZIP, nil
	case "snappy":
		return sarama.CompressionSnappy, nil
	case "lz4":
		return sarama.CompressionLZ4, nil
	case "zstd":
		return sarama.CompressionZSTD, nil
	default:
		return sarama.CompressionNone, fmt.Errorf("producer.compression should be one of 'none', 'gzip', 'snappy', 'lz4', or 'zstd'. configured value %v", compression)
	}
}

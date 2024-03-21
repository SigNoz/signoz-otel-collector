package signozkafkaexporter

import (
	"context"
	"fmt"

	"github.com/Shopify/sarama"
	"go.opentelemetry.io/collector/client"
)

// getKafkaTopicFromClientMetadata returns the kafka topic from client metadata
func getKafkaTopicFromClientMetadata(md client.Metadata) (string, error) {
	// return default topic if no tenant id is found in client metadata
	signozTenantId := md.Get("signoz_tenant_id")
	if len(signozTenantId) == 0 {
		return "", fmt.Errorf("signoz_tenant_id not found in client metadata")
	}
	return signozTenantId[0], nil
}

// getKafkaTopicFromClientMetadata returns the kafka topic from client metadata
func getRequestIdFromClientMetadata(md client.Metadata) (string, error) {
	// return default topic if no tenant id is found in client metadata
	signozTenantId := md.Get("signoz_request_id")
	if len(signozTenantId) == 0 {
		return "", fmt.Errorf("signoz_request_id not found in client metadata")
	}
	return signozTenantId[0], nil
}

func setRequestIdAsKey(ctx context.Context, messages []*sarama.ProducerMessage) ([]*sarama.ProducerMessage, error) {
	requestId, err := getRequestIdFromClientMetadata(client.FromContext(ctx).Metadata)
	if err != nil {
		return nil, err
	}
	for _, message := range messages {
		message.Key = sarama.StringEncoder(requestId)
	}
	return messages, nil
}

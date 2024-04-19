package signozkafkaexporter

import (
	"context"

	"github.com/SigNoz/signoz-otel-collector/pkg/entity"
)

const (
	DefaultKafkaTopicPrefix = "default"
)

// getKafkaTopicFromClientMetadata returns the kafka topic from client metadata
func getKafkaTopicPrefixFromCtx(ctx context.Context) string {
	// Retrieve credential id from ctx
	auth, ok := entity.AuthFromContext(ctx)
	if !ok {
		// If no auth was found, return the deafult topic.
		return DefaultKafkaTopicPrefix
	}

	return auth.TenantName()
}

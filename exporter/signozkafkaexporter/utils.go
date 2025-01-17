package signozkafkaexporter

import (
	"go.opentelemetry.io/collector/client"
)

const (
	DefaultKafkaTopicPrefix = "default"
)

// getKafkaTopicFromClientMetadata returns the kafka topic from client metadata
func getKafkaTopicPrefixFromClientMetadata(md client.Metadata) string {
	// return default topic if no tenant id is found in client metadata
	signozTenantId := md.Get("signoz_tenant_id")
	if len(signozTenantId) != 0 {
		return signozTenantId[0]
	}

	return DefaultKafkaTopicPrefix
}

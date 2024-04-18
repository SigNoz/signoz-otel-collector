package signozkafkaexporter

import "go.opentelemetry.io/collector/client"

const (
	DefaultKafkaTopicPrefix = "default"
)

// getKafkaTopicFromClientMetadata returns the kafka topic from client metadata
func getKafkaTopicPrefixFromClient(info client.Info) string {
	// return default topic if no tenant id is found in client metadata
	signozTenantId := info.Auth.GetAttribute("tenant.name")
	if signozTenantId == nil {
		return DefaultKafkaTopicPrefix
	}

	return signozTenantId.(string)
}

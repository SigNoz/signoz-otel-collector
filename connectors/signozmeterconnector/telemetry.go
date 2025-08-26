package signozmeterconnector

import (
	"context"

	"github.com/SigNoz/signoz-otel-collector/connectors/signozmeterconnector/internal/metadata"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	acceptedAttribute string = "accepted"
	sentAttribute     string = "sent"
	signalAttribute   string = "signal"
)

var (
	connectorRoleReceiver connectorRoles = "receiver"
	connectorRoleExporter connectorRoles = "exporter"
)

type connectorRoles string

type meterTelemetry struct {
	attributes []attribute.KeyValue
	counters   map[connectorRoles]metric.Int64Counter
}

func newMeterTelemetry(set connector.Settings) (*meterTelemetry, error) {
	telemetryBuilder, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	if err != nil {
		return nil, err
	}

	counters := make(map[connectorRoles]metric.Int64Counter)
	counters[connectorRoleReceiver] = telemetryBuilder.ConnectorReceivedItemsCount
	counters[connectorRoleExporter] = telemetryBuilder.ConnectorProducedItemsCount

	return &meterTelemetry{
		attributes: []attribute.KeyValue{
			attribute.String("connector", set.ID.String()),
		},
		counters: counters,
	}, nil
}

func (telemetry *meterTelemetry) record(ctx context.Context, role connectorRoles, value int64, attrs ...attribute.KeyValue) {
	telemetry.
		counters[role].
		Add(
			ctx,
			value,
			metric.WithAttributes(append(telemetry.attributes, attrs...)...),
		)
}

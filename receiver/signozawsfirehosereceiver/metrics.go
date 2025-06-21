// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signozawsfirehosereceiver // import "github.com/SigNoz/signoz-otel-collector/receiver/signozawsfirehosereceiver"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"google.golang.org/grpc/codes"

	"github.com/SigNoz/signoz-otel-collector/config/configrouter"
	"github.com/SigNoz/signoz-otel-collector/receiver/signozawsfirehosereceiver/internal/metadata"
	"github.com/SigNoz/signoz-otel-collector/receiver/signozawsfirehosereceiver/internal/unmarshaler"
	"github.com/SigNoz/signoz-otel-collector/receiver/signozawsfirehosereceiver/internal/unmarshaler/cwmetricstream"
)

const defaultMetricsRecordType = cwmetricstream.TypeStr

// The metricsConsumer implements the firehoseConsumer
// to use a metrics consumer and unmarshaler.
type metricsConsumer struct {
	// consumer passes the translated metrics on to the
	// next consumer.
	consumer consumer.Metrics
	// unmarshaler is the configured MetricsUnmarshaler
	// to use when processing the records.
	unmarshaler unmarshaler.MetricsUnmarshaler
	// obsreport
	obsreport *receiverhelper.ObsReport
}

var _ firehoseConsumer = (*metricsConsumer)(nil)

// newMetricsReceiver creates a new instance of the receiver
// with a metricsConsumer.
func newMetricsReceiver(
	config *Config,
	set receiver.Settings,
	unmarshalers map[string]unmarshaler.MetricsUnmarshaler,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	recordType := config.RecordType
	if recordType == "" {
		recordType = defaultMetricsRecordType
	}
	configuredUnmarshaler := unmarshalers[recordType]
	if configuredUnmarshaler == nil {
		return nil, fmt.Errorf("%w: recordType = %s", errUnrecognizedRecordType, recordType)
	}

	transport := "http"
	if config.TLS != nil {
		transport = "https"
	}

	obsreport, err := receiverhelper.NewObsReport(
		receiverhelper.ObsReportSettings{
			ReceiverID:             set.ID,
			Transport:              transport,
			ReceiverCreateSettings: set,
		},
	)
	if err != nil {
		return nil, err
	}

	mc := &metricsConsumer{
		consumer:    nextConsumer,
		unmarshaler: configuredUnmarshaler,
		obsreport:   obsreport,
	}

	return &firehoseReceiver{
		settings: set,
		config:   config,
		consumer: mc,
	}, nil
}

// Consume uses the configured unmarshaler to deserialize the records into a
// single pmetric.Metrics. If there are common attributes available, then it will
// attach those to each of the pcommon.Resources. It will send the final result
// to the next consumer.
func (mc *metricsConsumer) Consume(ctx context.Context, records [][]byte, commonAttributes map[string]string) error {
	md, err := mc.unmarshaler.Unmarshal(records)
	if err != nil {
		return configrouter.FromError(err, codes.InvalidArgument)
	}

	if commonAttributes != nil {
		for i := 0; i < md.ResourceMetrics().Len(); i++ {
			rm := md.ResourceMetrics().At(i)
			for k, v := range commonAttributes {
				if _, found := rm.Resource().Attributes().Get(k); !found {
					rm.Resource().Attributes().PutStr(k, v)
				}
			}
		}
	}

	numMetrics := md.DataPointCount()

	ctx = mc.obsreport.StartMetricsOp(ctx)
	err = mc.consumer.ConsumeMetrics(ctx, md)
	mc.obsreport.EndMetricsOp(ctx, metadata.Type.String(), numMetrics, err)
	if err != nil {
		return configrouter.FromError(err, codes.Internal)
	}

	return nil
}

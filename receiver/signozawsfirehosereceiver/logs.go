// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signozawsfirehosereceiver // import "github.com/SigNoz/signoz-otel-collector/receiver/signozawsfirehosereceiver"

import (
	"context"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"google.golang.org/grpc/codes"

	"github.com/SigNoz/signoz-otel-collector/config/configrouter"
	"github.com/SigNoz/signoz-otel-collector/receiver/signozawsfirehosereceiver/internal/metadata"
	"github.com/SigNoz/signoz-otel-collector/receiver/signozawsfirehosereceiver/internal/unmarshaler"
	"github.com/SigNoz/signoz-otel-collector/receiver/signozawsfirehosereceiver/internal/unmarshaler/cwlog"
)

const defaultLogsRecordType = cwlog.TypeStr

// logsConsumer implements the firehoseConsumer
// to use a logs consumer and unmarshaler.
type logsConsumer struct {
	// consumer passes the translated logs on to the
	// next consumer.
	consumer consumer.Logs
	// unmarshaler is the configured LogsUnmarshaler
	// to use when processing the records.
	unmarshaler unmarshaler.LogsUnmarshaler
	// obsreport
	obsreport *receiverhelper.ObsReport
}

var _ firehoseConsumer = (*logsConsumer)(nil)

// newLogsReceiver creates a new instance of the receiver
// with a logsConsumer.
func newLogsReceiver(
	config *Config,
	set receiver.Settings,
	unmarshalers map[string]unmarshaler.LogsUnmarshaler,
	nextConsumer consumer.Logs,
) (receiver.Logs, error) {
	recordType := config.RecordType
	if recordType == "" {
		recordType = defaultLogsRecordType
	}
	configuredUnmarshaler := unmarshalers[recordType]
	if configuredUnmarshaler == nil {
		return nil, errUnrecognizedRecordType
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

	mc := &logsConsumer{
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
// single plog.Logs. It will send the final result
// to the next consumer.
func (mc *logsConsumer) Consume(ctx context.Context, records [][]byte, commonAttributes map[string]string) error {
	md, err := mc.unmarshaler.Unmarshal(records)
	if err != nil {
		return configrouter.FromError(err, codes.InvalidArgument)
	}

	if commonAttributes != nil {
		for i := 0; i < md.ResourceLogs().Len(); i++ {
			rm := md.ResourceLogs().At(i)
			for k, v := range commonAttributes {
				if _, found := rm.Resource().Attributes().Get(k); !found {
					rm.Resource().Attributes().PutStr(k, v)
				}
			}
		}
	}

	numLogs := md.LogRecordCount()

	ctx = mc.obsreport.StartLogsOp(ctx)
	err = mc.consumer.ConsumeLogs(ctx, md)
	mc.obsreport.EndLogsOp(ctx, metadata.Type.String(), numLogs, err)
	if err != nil {
		return configrouter.FromError(err, codes.Internal)
	}

	return nil
}

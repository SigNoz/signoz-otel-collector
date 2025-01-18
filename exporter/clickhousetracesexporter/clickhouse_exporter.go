// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package clickhousetracesexporter

import (
	"context"
	"io"
	"sync"

	"github.com/SigNoz/signoz-otel-collector/usage"
	"github.com/SigNoz/signoz-otel-collector/utils"
	"github.com/google/uuid"
	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.5.0"
	"go.uber.org/zap"
)

const (
	hasIsRemoteMask uint32 = 0x00000100
	isRemoteMask    uint32 = 0x00000200
)

// Crete new exporter.
func newExporter(cfg component.Config, logger *zap.Logger, settings exporter.Settings) (*storage, error) {

	if err := component.ValidateConfig(cfg); err != nil {
		return nil, err
	}

	configClickHouse := cfg.(*Config)

	id := uuid.New()

	f := ClickHouseNewFactory(id, *configClickHouse, settings)

	err := f.Initialize(logger)
	if err != nil {
		return nil, err
	}
	spanWriter, err := f.CreateSpanWriter()
	if err != nil {
		return nil, err
	}

	collector := usage.NewUsageCollector(
		id,
		f.db,
		usage.Options{ReportingInterval: usage.DefaultCollectionInterval},
		"signoz_traces",
		UsageExporter,
	)
	collector.Start()

	if err := view.Register(SpansCountView, SpansCountBytesView); err != nil {
		return nil, err
	}

	storage := storage{
		id:             id,
		Writer:         spanWriter,
		usageCollector: collector,
		config: storageConfig{
			lowCardinalExceptionGrouping: configClickHouse.LowCardinalExceptionGrouping,
		},
		wg:           new(sync.WaitGroup),
		closeChan:    make(chan struct{}),
		useNewSchema: configClickHouse.UseNewSchema,
		logger:       logger,
	}

	return &storage, nil
}

type storage struct {
	id             uuid.UUID
	Writer         Writer
	usageCollector *usage.UsageCollector
	config         storageConfig
	wg             *sync.WaitGroup
	closeChan      chan struct{}
	useNewSchema   bool
	logger         *zap.Logger
}

type storageConfig struct {
	lowCardinalExceptionGrouping bool
}

func makeJaegerProtoReferences(
	links ptrace.SpanLinkSlice,
	parentSpanID pcommon.SpanID,
	traceID pcommon.TraceID,
) ([]OtelSpanRef, error) {

	parentSpanIDSet := len([8]byte(parentSpanID)) != 0
	if !parentSpanIDSet && links.Len() == 0 {
		return nil, nil
	}

	refsCount := links.Len()
	if parentSpanIDSet {
		refsCount++
	}

	refs := make([]OtelSpanRef, 0, refsCount)

	// Put parent span ID at the first place because usually backends look for it
	// as the first CHILD_OF item in the model.SpanRef slice.
	if parentSpanIDSet {

		refs = append(refs, OtelSpanRef{
			TraceId: utils.TraceIDToHexOrEmptyString(traceID),
			SpanId:  utils.SpanIDToHexOrEmptyString(parentSpanID),
			RefType: "CHILD_OF",
		})
	}

	for i := 0; i < links.Len(); i++ {
		link := links.At(i)

		refs = append(refs, OtelSpanRef{
			TraceId: utils.TraceIDToHexOrEmptyString(link.TraceID()),
			SpanId:  utils.SpanIDToHexOrEmptyString(link.SpanID()),

			// Since Jaeger RefType is not captured in internal data,
			// use SpanRefType_FOLLOWS_FROM by default.
			// SpanRefType_CHILD_OF supposed to be set only from parentSpanID.
			RefType: "FOLLOWS_FROM",
		})
	}

	return refs, nil
}

// ServiceNameForResource gets the service name for a specified Resource.
// TODO: Find a better package for this function.
func ServiceNameForResource(resource pcommon.Resource) string {

	service, found := resource.Attributes().Get(conventions.AttributeServiceName)
	if !found {
		return "<nil-service-name>"
	}

	return service.Str()
}

func (s *storage) pushTraceData(ctx context.Context, td ptrace.Traces) error {
	return s.pushTraceDataV3(ctx, td)
}

// Shutdown will shutdown the exporter.
func (s *storage) Shutdown(_ context.Context) error {

	close(s.closeChan)
	s.wg.Wait()

	if s.usageCollector != nil {
		s.usageCollector.Stop()
	}

	if closer, ok := s.Writer.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

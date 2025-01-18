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
	"github.com/google/uuid"
	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

const (
	hasIsRemoteMask          uint32 = 0x00000100
	isRemoteMask             uint32 = 0x00000200
	defaultDatasource        string = "tcp://127.0.0.1:9000/?database=signoz_traces"
	defaultTraceDatabase     string = "signoz_traces"
	defaultErrorTable        string = "distributed_signoz_error_index_v2"
	defaultAttributeTableV2  string = "distributed_tag_attributes_v2"
	defaultAttributeKeyTable string = "distributed_span_attributes_keys"
	defaultIndexTableV3      string = "distributed_signoz_index_v3"
	defaultResourceTableV3   string = "distributed_traces_v3_resource"
	insertTraceSQLTemplateV2        = `INSERT INTO %s.%s (
		ts_bucket_start,
		resource_fingerprint,
		timestamp,
		trace_id,
		span_id,
		trace_state,
		parent_span_id,
		flags,
		name,
		kind,
		kind_string,
		duration_nano,
		status_code,
		status_message,
		status_code_string,
		attributes_string,
		attributes_number,
		attributes_bool,
		resources_string,
		events,
		links,
		response_status_code,
		external_http_url,
		http_url,
		external_http_method,
		http_method,
		http_host,
		db_name,
		db_operation,
		has_error,
		is_remote
		) VALUES (
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?,
			?
			)`
)

type clickhouseTracesExporter struct {
	id             uuid.UUID
	Writer         Writer
	usageCollector *usage.UsageCollector
	config         storageConfig
	wg             *sync.WaitGroup
	closeChan      chan struct{}
}

type storageConfig struct {
	lowCardinalExceptionGrouping bool
}

// Crete new exporter.
func newExporter(cfg *Config, _ exporter.Settings, writerOpts []WriterOption, exporterOpts []TraceExporterOption) (*clickhouseTracesExporter, error) {

	if err := view.Register(SpansCountView, SpansCountBytesView); err != nil {
		return nil, err
	}

	writer := NewSpanWriter(writerOpts...)

	exporter := clickhouseTracesExporter{
		Writer: writer,
		config: storageConfig{
			lowCardinalExceptionGrouping: cfg.LowCardinalExceptionGrouping,
		},
		wg:        new(sync.WaitGroup),
		closeChan: make(chan struct{}),
	}

	for _, opt := range exporterOpts {
		opt(&exporter)
	}

	return &exporter, nil
}

func (s *clickhouseTracesExporter) pushTraceData(ctx context.Context, td ptrace.Traces) error {
	return s.pushTraceDataV3(ctx, td)
}

func (s *clickhouseTracesExporter) Shutdown(_ context.Context) error {

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

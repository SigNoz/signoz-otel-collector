// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package clickhousetracesexporter

import (
	"context"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/SigNoz/signoz-otel-collector/usage"
	"github.com/google/uuid"
	"go.opentelemetry.io/collector/exporter"
	metricapi "go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

// Factory implements storage.Factory for Clickhouse backend.
type Factory struct {
	logger     *zap.Logger
	meter      metricapi.Meter
	Options    *Options
	db         clickhouse.Conn
	archive    clickhouse.Conn
	makeWriter writerMaker
}

// Writer writes spans to storage.
type Writer interface {
	WriteBatchOfSpans(ctx context.Context, span []*Span) error
	WriteBatchOfSpansV3(ctx context.Context, span []*SpanV3, metrics map[string]usage.Metric) error
	WriteResourcesV3(ctx context.Context, resourcesSeen map[int64]map[string]string) error
}

type writerMaker func(WriterOptions) (Writer, error)

// NewFactory creates a new Factory.
func ClickHouseNewFactory(exporterId uuid.UUID, config Config, settings exporter.Settings) *Factory {

	return &Factory{
		meter:   settings.MeterProvider.Meter("github.com/SigNoz/signoz-otel-collector/exporter/clickhousetracesexporter"),
		Options: NewOptions(exporterId, config, primaryNamespace, config.UseNewSchema, archiveNamespace),
		// makeReader: func(db *clickhouse.Conn, operationsTable, indexTable, spansTable string) (spanstore.Reader, error) {
		// 	return store.NewTraceReader(db, operationsTable, indexTable, spansTable), nil
		// },
		makeWriter: func(options WriterOptions) (Writer, error) {
			return NewSpanWriter(options), nil
		},
	}
}

// Initialize implements storage.Factory
func (f *Factory) Initialize(logger *zap.Logger) error {
	f.logger = logger

	db, err := f.connect(f.Options.getPrimary())
	if err != nil {
		return fmt.Errorf("error connecting to primary db: %v", err)
	}

	f.db = db

	archiveConfig := f.Options.others[archiveNamespace]
	if archiveConfig.Enabled {
		archive, err := f.connect(archiveConfig)
		if err != nil {
			return fmt.Errorf("error connecting to archive db: %v", err)
		}

		f.archive = archive
	}
	return nil
}

func (f *Factory) connect(cfg *namespaceConfig) (clickhouse.Conn, error) {
	if cfg.Encoding != EncodingJSON && cfg.Encoding != EncodingProto {
		return nil, fmt.Errorf("unknown encoding %q, supported: %q, %q", cfg.Encoding, EncodingJSON, EncodingProto)
	}

	return cfg.Connector(cfg)
}

// CreateSpanWriter implements storage.Factory
func (f *Factory) CreateSpanWriter() (Writer, error) {
	cfg := f.Options.getPrimary()
	return f.makeWriter(WriterOptions{
		logger:            f.logger,
		meter:             f.meter,
		db:                f.db,
		traceDatabase:     cfg.TraceDatabase,
		spansTable:        cfg.SpansTable,
		indexTable:        cfg.IndexTable,
		errorTable:        cfg.ErrorTable,
		attributeTable:    cfg.AttributeTable,
		attributeTableV2:  cfg.AttributeTableV2,
		attributeKeyTable: cfg.AttributeKeyTable,
		encoding:          cfg.Encoding,
		exporterId:        cfg.ExporterId,

		useNewSchema:      cfg.UseNewSchema,
		indexTableV3:      cfg.IndexTableV3,
		resourceTableV3:   cfg.ResourceTableV3,
		maxDistinctValues: cfg.MaxDistinctValues,
		fetchKeysInterval: cfg.FetchKeysInterval,
	})
}

// CreateArchiveSpanWriter implements storage.ArchiveFactory
func (f *Factory) CreateArchiveSpanWriter() (Writer, error) {
	if f.archive == nil {
		return nil, nil
	}
	cfg := f.Options.others[archiveNamespace]
	return f.makeWriter(WriterOptions{
		logger:            f.logger,
		db:                f.archive,
		traceDatabase:     cfg.TraceDatabase,
		spansTable:        cfg.SpansTable,
		indexTable:        cfg.IndexTable,
		errorTable:        cfg.ErrorTable,
		attributeTable:    cfg.AttributeTable,
		attributeTableV2:  cfg.AttributeTableV2,
		attributeKeyTable: cfg.AttributeKeyTable,
		encoding:          cfg.Encoding,
		exporterId:        cfg.ExporterId,

		useNewSchema:      cfg.UseNewSchema,
		indexTableV3:      cfg.IndexTableV3,
		resourceTableV3:   cfg.ResourceTableV3,
		maxDistinctValues: cfg.MaxDistinctValues,
		fetchKeysInterval: cfg.FetchKeysInterval,
	})
}

// Close Implements io.Closer and closes the underlying storage
func (f *Factory) Close() error {
	if f.db != nil {
		err := f.db.Close()
		if err != nil {
			return err
		}

		f.db = nil
	}

	if f.archive != nil {
		err := f.archive.Close()
		if err != nil {
			return err
		}

		f.archive = nil
	}

	return nil
}
